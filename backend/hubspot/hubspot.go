package hubspot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	kafka_queue "github.com/highlight-run/highlight/backend/kafka-queue"
	"github.com/highlight-run/highlight/backend/redis"
	"github.com/openlyinc/pointy"
	"github.com/samber/lo"

	"github.com/aws/smithy-go/ptr"
	"github.com/goware/emailproviders"
	"github.com/highlight-run/highlight/backend/model"
	"github.com/leonelquinteros/hubspot"
	e "github.com/pkg/errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const Retries = 5

// ClientSideContactCreationTimeout is the time we will wait for the object to be created by the hubspot client-side snippet
const ClientSideContactCreationTimeout = 3 * time.Minute

// ClientSideCompanyCreationTimeout is double the contact creation time because we expect contact creation to create a company.
// The company creation backend task can be kicked off at the same time that contact creation is kicked off, so
// we want to wait (in the worst-case) for the contact creation to time out, manually create a contact, and then
// wait for the company creation to time out with the same delay of 3 minutes.
const ClientSideCompanyCreationTimeout = 2 * ClientSideContactCreationTimeout

const ClientSideCreationPollInterval = 5 * time.Second

var (
	OAuthToken = os.Getenv("HUBSPOT_OAUTH_TOKEN")
	APIKey     = os.Getenv("HUBSPOT_API_KEY")
	// APICookie and CSRFToken are reverse engineered from the frontend request flow.
	// they only need to be set for the doppelgänger functionality.
	APICookie = os.Getenv("HUBSPOT_API_COOKIE")
	CSRFToken = os.Getenv("HUBSPOT_CSRF_TOKEN")
)

func retry[T *int](fn func() (T, error)) (ret T, err error) {
	for i := 0; i < Retries; i++ {
		ret, err = fn()
		if err == nil {
			return
		}
		time.Sleep(16 * time.Millisecond * time.Duration(math.Pow(2, float64(i))))
	}
	return
}

func pollHubspot[T *int](fn func() (T, error), timeout time.Duration) (result T, err error) {
	start := time.Now()
	ticker := time.NewTicker(ClientSideCreationPollInterval)
	defer ticker.Stop()
	for t := range ticker.C {
		result, err = fn()
		if result != nil {
			return
		}
		if t.Sub(start) > timeout {
			break
		}
	}
	return
}

type HubspotApi struct {
	db            *gorm.DB
	hubspotClient hubspot.Client
	redisClient   *redis.Client
	kafkaProducer *kafka_queue.Queue
}

func NewHubspotAPI(client hubspot.Client, db *gorm.DB, redisClient *redis.Client, kafkaProducer *kafka_queue.Queue) *HubspotApi {
	return &HubspotApi{
		db:            db,
		hubspotClient: client,
		redisClient:   redisClient,
		kafkaProducer: kafkaProducer,
	}
}

// this is taken/edits from https://sourcegraph.com/github.com/leonelquinteros/hubspot/-/blob/contacts.go, but I got rid of 'AssociatedCompany' because of errors.
type CustomContactsResponse struct {
	PortalID     int    `json:"portal-id"`
	Vid          int    `json:"vid"`
	CanonicalVid int    `json:"canonical-vid"`
	MergeVids    []int  `json:"merge-vids"`
	IsContact    bool   `json:"is-contact"`
	ProfileToken string `json:"profile-token"`
	ProfileURL   string `json:"profile-url"`
}

type CompanyProperty struct {
	Value     string `json:"value"`
	Timestamp int64  `json:"timestamp"`
}

type CompanyResponse struct {
	PortalID   int                         `json:"portalId"`
	CompanyID  int                         `json:"companyId"`
	Properties map[string]*CompanyProperty `json:"properties"`
}

type CompaniesResponse struct {
	Results []*CompanyResponse `json:"results"`
	HasMore bool               `json:"hasMore"`
	Offset  int                `json:"offset"`
	Total   int                `json:"total"`
}

type DoppelgangersPropertyVersion struct {
	Name      string   `json:"name"`
	Source    string   `json:"source"`
	SourceID  string   `json:"sourceId"`
	SourceVid []string `json:"sourceVid"`
	Timestamp int      `json:"timestamp"`
	Value     string   `json:"value"`
}

type DoppelgangersProperty struct {
	PersistenceTimestamp *int                            `json:"persistenceTimestamp"`
	Source               string                          `json:"source"`
	SourceID             string                          `json:"sourceId"`
	Timestamp            *int                            `json:"timestamp"`
	UpdatedByUserID      *int                            `json:"updatedByUserId"`
	Value                *string                         `json:"value"`
	Versions             []*DoppelgangersPropertyVersion `json:"versions"`
}

type DoppelgangersObject struct {
	ObjectType string                            `json:"objectType"`
	ObjectID   int                               `json:"objectId"`
	PortalID   int                               `json:"portalId"`
	Properties map[string]*DoppelgangersProperty `json:"properties"`
}

type DoppelgangersResult struct {
	FeedbackID      string  `json:"feedbackId"`
	ID1             int     `json:"id1"`
	ID2             int     `json:"id2"`
	SimilarityScore float64 `json:"similarityScore"`
}

type DoppelgangersUserSavedPropertyOption struct {
	DisplayOrder int    `json:"displayOrder"`
	Hidden       bool   `json:"hidden"`
	Label        string `json:"label"`
	ReadOnly     bool   `json:"readOnly"`
	Value        string `json:"value"`
}

type DoppelgangersUserSavedProperty struct {
	ExternalOptionsReferenceType *string                                 `json:"externalOptionsReferenceType"`
	FieldType                    string                                  `json:"fieldType"`
	Hidden                       bool                                    `json:"hidden"`
	HubspotDefined               bool                                    `json:"hubspotDefined"`
	Label                        string                                  `json:"label"`
	Name                         string                                  `json:"name"`
	Type                         string                                  `json:"type"`
	Options                      []*DoppelgangersUserSavedPropertyOption `json:"options"`
}

type DoppelgangersResponse struct {
	Results             []*DoppelgangersResult            `json:"results"`
	Objects             map[int]*DoppelgangersObject      `json:"objects"`
	UserSavedProperties []*DoppelgangersUserSavedProperty `json:"userSavedProperties"`
	HasMore             bool                              `json:"hasMore"`
	Offset              int                               `json:"offset"`
	Total               int                               `json:"total"`
	LastScoredTimestamp int64                             `json:"lastScoredTimestamp"`
}

func (h *HubspotApi) doRequest(url string, result interface{}, params map[string]string, method string, b io.Reader) error {
	req, _ := http.NewRequest(method, fmt.Sprintf("https://api.hubapi.com%s", url), b)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+OAuthToken)
	q := req.URL.Query()
	q.Add("hapikey", APIKey)
	for k, v := range params {
		q.Add(k, v)
	}
	if APICookie != "" {
		req.AddCookie(&http.Cookie{Name: "hubspotapi", Value: APICookie})
	}
	if CSRFToken != "" {
		req.Header.Add("X-Hubspot-Csrf-Hubspotapi", CSRFToken)
		req.AddCookie(&http.Cookie{Name: "hubspotapi-csrf", Value: CSRFToken})
		req.AddCookie(&http.Cookie{Name: "csrf.app", Value: CSRFToken})
	}
	req.URL.RawQuery = q.Encode()

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if result != nil {
		err = json.Unmarshal(body, &result)
		if err != nil {
			return err
		}
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("HubSpot API error: %d - %s \n%s", resp.StatusCode, resp.Status, string(body))
	}

	return nil
}

func (h *HubspotApi) getDoppelgangers(ctx context.Context) (results []*DoppelgangersResult, objects map[int]*DoppelgangersObject, err error) {
	objects = make(map[int]*DoppelgangersObject)
	for {
		r := DoppelgangersResponse{}
		if err = h.doRequest("/doppelganger/v1/similar/company/resultPage", &r, map[string]string{
			"pageSize":   "50",
			"offset":     strconv.Itoa(len(results)),
			"properties": "name,domain,hs_num_child_companies,hs_parent_company_id,hs_last_sales_activity_timestamp,createdate,hs_lastmodifieddate",
			"portalId":   "20473940",
		}, "GET", nil); err != nil {
			return
		} else {
			for k, v := range r.Objects {
				objects[k] = v
			}
			results = append(results, r.Results...)
			if !r.HasMore {
				break
			}
		}
	}
	return
}

func (h *HubspotApi) mergeCompanies(keepID, mergeID int) error {
	return h.doRequest(fmt.Sprintf("/companies/v2/companies/%d/merge", keepID), nil, map[string]string{
		"portalId": "20473940",
	}, "PUT", strings.NewReader(fmt.Sprintf(`{"companyIdToMerge":%d}`, mergeID)))
}

func (h *HubspotApi) getAllCompanies(ctx context.Context) (companies []*CompanyResponse, err error) {
	if h.redisClient != nil {
		err = h.redisClient.GetHubspotCompanies(ctx, &companies)
		if err == nil && len(companies) > 0 {
			return companies, nil
		}
	}
	for {
		r := CompaniesResponse{}
		if err = h.doRequest("/companies/v2/companies/recent/created", &r, map[string]string{
			"count": "100", "offset": strconv.Itoa(len(companies)),
		}, "GET", nil); err != nil {
			return
		} else {
			companies = append(companies, r.Results...)
			if !r.HasMore {
				break
			}
		}
	}
	if h.redisClient != nil {
		_ = h.redisClient.SetHubspotCompanies(ctx, &companies)
	}
	return
}

func (h *HubspotApi) getCompany(ctx context.Context, name, domain string) (*int, error) {
	companies, err := h.getAllCompanies(ctx)
	if err != nil {
		return nil, err
	}
	if company, ok := lo.Find(companies, func(response *CompanyResponse) bool {
		for prop, data := range response.Properties {
			if domain != "" && prop == "domain" {
				return strings.EqualFold(data.Value, domain)
			} else if name != "" && prop == "name" {
				return strings.EqualFold(data.Value, name)
			}
		}
		return false
	}); !ok {
		return nil, e.New(fmt.Sprintf("failed to find company with name %s domain %s", name, domain))
	} else {
		return pointy.Int(company.CompanyID), nil
	}
}

func (h *HubspotApi) getContactForAdmin(email string) (contactId *int, err error) {
	r := CustomContactsResponse{}
	if err := h.hubspotClient.Contacts().Client.Request("GET", "/contacts/v1/contact/email/"+email+"/profile", nil, &r); err != nil {
		return nil, err
	} else {
		return pointy.Int(r.Vid), nil
	}
}

func (h *HubspotApi) createContactForAdmin(ctx context.Context, email string, userDefinedRole string, userDefinedPersona string, first string, last string, phone string, referral string) (contactId *int, err error) {
	var hubspotContactId int
	if resp, err := h.hubspotClient.Contacts().Create(hubspot.ContactsRequest{
		Properties: []hubspot.Property{
			{
				Property: "email",
				Name:     "email",
				Value:    email,
			},
			{
				Property: "user_defined_role",
				Name:     "user_defined_role",
				Value:    userDefinedRole,
			},
			{
				Property: "user_defined_persona",
				Name:     "user_defined_persona",
				Value:    userDefinedPersona,
			},
			{
				Property: "firstname",
				Name:     "firstname",
				Value:    first,
			},
			{
				Property: "lastname",
				Name:     "lastname",
				Value:    last,
			},
			{
				Property: "phone",
				Name:     "phone",
				Value:    phone,
			},
			{
				Property: "referral_url",
				Name:     "referral_url",
				Value:    referral,
			},
		},
	}); err != nil {
		// If there's an error creating the contact, assume its a conflict and try to get the existing user.
		if contact, err := h.getContactForAdmin(email); err == nil {
			return contact, nil
		}
	} else {
		hubspotContactId = resp.Vid
	}
	log.WithContext(ctx).Infof("succesfully created a hubspot contact with id: %v", hubspotContactId)

	return &hubspotContactId, nil
}

func (h *HubspotApi) CreateContactCompanyAssociation(ctx context.Context, adminID int, workspaceID int) error {
	return h.kafkaProducer.Submit(ctx, &kafka_queue.Message{
		Type: kafka_queue.HubSpotCreateContactCompanyAssociation,
		HubSpotCreateContactCompanyAssociation: &kafka_queue.HubSpotCreateContactCompanyAssociationArgs{
			AdminID:     adminID,
			WorkspaceID: workspaceID,
		},
	}, "")
}

func (h *HubspotApi) CreateContactCompanyAssociationImpl(ctx context.Context, adminID int, workspaceID int) error {
	key := fmt.Sprintf("hubspot-association-%d-%d", adminID, workspaceID)
	// wait for up to 5 seconds in case another worker is creating the same association
	// we don't expect the action to take longer than 5 seconds
	if acquired := h.redisClient.AcquireLock(ctx, key, 5*time.Second); acquired {
		defer func() {
			if err := h.redisClient.ReleaseLock(ctx, key); err != nil {
				log.WithContext(ctx).WithError(err).WithField("url", key).Error("failed to release hubspot association lock")
			}
		}()
	}

	admin := &model.Admin{}
	if err := h.db.Model(&model.Admin{}).Where("id = ?", adminID).Take(&admin).Error; err != nil {
		return err
	}
	workspace := &model.Workspace{}
	if err := h.db.Model(&model.Workspace{}).Where("id = ?", workspaceID).Take(&workspace).Error; err != nil {
		return err
	}
	if workspace.HubspotCompanyID == nil {
		return e.New("hubspot company id is empty")
	} else if admin.HubspotContactID == nil {
		return e.New("hubspot contact id is empty")
	}
	if err := h.hubspotClient.CRMAssociations().Create(hubspot.CRMAssociationsRequest{
		DefinitionID: hubspot.CRMAssociationCompanyToContact,
		FromObjectID: *workspace.HubspotCompanyID,
		ToObjectID:   *admin.HubspotContactID,
	}); err != nil {
		return err
	} else {
		log.WithContext(ctx).Info("success creating company to contact association")
	}
	if err := h.hubspotClient.CRMAssociations().Create(hubspot.CRMAssociationsRequest{
		DefinitionID: hubspot.CRMAssociationContactToCompany,
		FromObjectID: *admin.HubspotContactID,
		ToObjectID:   *workspace.HubspotCompanyID,
	}); err != nil {
		return err
	} else {
		log.WithContext(ctx).Info("success creating contact to company association")
	}
	return nil
}

func (h *HubspotApi) CreateContactForAdmin(ctx context.Context, adminID int, email string, userDefinedRole string, userDefinedPersona string, first string, last string, phone string, referral string) error {
	return h.kafkaProducer.Submit(ctx, &kafka_queue.Message{
		Type: kafka_queue.HubSpotCreateContactForAdmin,
		HubSpotCreateContactForAdmin: &kafka_queue.HubSpotCreateContactForAdminArgs{
			AdminID:            adminID,
			Email:              email,
			UserDefinedRole:    userDefinedRole,
			UserDefinedPersona: userDefinedPersona,
			First:              first,
			Last:               last,
			Phone:              phone,
			Referral:           referral,
		},
	}, "")
}

func (h *HubspotApi) CreateContactForAdminImpl(ctx context.Context, adminID int, email string, userDefinedRole string, userDefinedPersona string, first string, last string, phone string, referral string) (contactId *int, err error) {
	key := fmt.Sprintf("contact-%s", email)
	if acquired := h.redisClient.AcquireLock(ctx, key, ClientSideContactCreationTimeout); acquired {
		defer func() {
			if err := h.redisClient.ReleaseLock(ctx, key); err != nil {
				log.WithContext(ctx).WithError(err).WithField("url", key).Error("failed to release hubspot contact creation lock")
			}
		}()
	}

	if contactId, err = pollHubspot(func() (*int, error) {
		return h.getContactForAdmin(email)
	}, ClientSideContactCreationTimeout); contactId == nil {
		log.WithContext(ctx).
			WithField("email", email).
			Warnf("failed to get client-side hubspot contact. creating")
		contactId, err = retry(func() (*int, error) {
			return h.createContactForAdmin(ctx, email, userDefinedRole, userDefinedPersona, first, last, phone, referral)
		})

		if err != nil || contactId == nil {
			return nil, err
		}
		log.WithContext(ctx).Infof("succesfully created a hubspot contact with id: %v", contactId)
	}

	if err := h.db.Model(&model.Admin{Model: model.Model{ID: adminID}}).
		Updates(&model.Admin{HubspotContactID: contactId}).Error; err != nil {
		return nil, err
	}
	return
}

func (h *HubspotApi) CreateCompanyForWorkspace(ctx context.Context, workspaceID int, adminEmail string, name string) error {
	return h.kafkaProducer.Submit(ctx, &kafka_queue.Message{
		Type: kafka_queue.HubSpotCreateCompanyForWorkspace,
		HubSpotCreateCompanyForWorkspace: &kafka_queue.HubSpotCreateCompanyForWorkspaceArgs{
			WorkspaceID: workspaceID,
			AdminEmail:  adminEmail,
			Name:        name,
		},
	}, "")
}

func (h *HubspotApi) CreateCompanyForWorkspaceImpl(ctx context.Context, workspaceID int, adminEmail string, name string) (companyID *int, err error) {
	// Don't create for Demo account
	if workspaceID == 0 {
		return
	}

	key := fmt.Sprintf("company-%s", name)
	if acquired := h.redisClient.AcquireLock(ctx, key, ClientSideCompanyCreationTimeout); acquired {
		defer func() {
			if err := h.redisClient.ReleaseLock(ctx, key); err != nil {
				log.WithContext(ctx).WithError(err).WithField("url", key).Error("failed to release hubspot company creation lock")
			}
		}()
	}

	workspace := &model.Workspace{}
	if err := h.db.Model(&model.Workspace{}).Where("id = ?", workspaceID).Take(&workspace).Error; err != nil {
		return nil, err
	}

	var domain string
	if !emailproviders.Exists(adminEmail) {
		components := strings.Split(adminEmail, "@")
		if len(components) > 1 {
			domain = components[1]
		}
	}
	hexLink := fmt.Sprintf("https://workspace-details.highlight.io?_workspace_id=%v", workspaceID)
	companyProperties := hubspot.CompaniesRequest{
		Properties: []hubspot.Property{
			{
				Property: "name",
				Name:     "name",
				Value:    name,
			},
			{
				Property: "domain",
				Name:     "domain",
				Value:    domain,
			},
			{
				Property: "hex_link",
				Name:     "hex_link",
				Value:    hexLink,
			},
		},
	}
	if workspace.TrialEndDate != nil {
		companyProperties.Properties = append(companyProperties.Properties, hubspot.Property{
			Property: "trial_end_date",
			Name:     "trial_end_date",
			Value:    *workspace.TrialEndDate,
		})
	}

	if companyID, err = pollHubspot(func() (*int, error) {
		return h.getCompany(ctx, name, domain)
	}, ClientSideCompanyCreationTimeout); companyID != nil {
		log.WithContext(ctx).
			WithField("name", name).
			WithField("domain", domain).
			Infof("company already exists in Hubspot. updating")

		if _, err := h.hubspotClient.Companies().Update(*companyID, companyProperties); err != nil {
			return nil, err
		}
	} else {
		log.WithContext(ctx).
			WithField("name", name).
			Warnf("failed to get client-side hubspot company. creating")

		resp, err := h.hubspotClient.Companies().Create(companyProperties)
		if err != nil {
			return nil, err
		}
		companyID = &resp.CompanyID
		log.WithContext(ctx).Infof("succesfully created a hubspot company with id: %v", resp.CompanyID)
	}

	if err = h.db.Model(&model.Workspace{Model: model.Model{ID: workspaceID}}).
		Updates(&model.Workspace{HubspotCompanyID: companyID}).Error; err != nil {
		return
	}

	if adminEmail != "" {
		admin := model.Admin{}
		if err = h.db.Model(&model.Admin{}).Where(&model.Admin{Email: &adminEmail}).Take(&admin).Error; err != nil {
			return
		}
		if err := h.CreateContactCompanyAssociation(ctx, admin.ID, workspaceID); err != nil {
			return companyID, err
		}
	}
	return
}

func (h *HubspotApi) UpdateContactProperty(ctx context.Context, adminID int, properties []hubspot.Property) error {
	return h.kafkaProducer.Submit(ctx, &kafka_queue.Message{
		Type: kafka_queue.HubSpotUpdateContactProperty,
		HubSpotUpdateContactProperty: &kafka_queue.HubSpotUpdateContactPropertyArgs{
			AdminID:    adminID,
			Properties: properties,
		},
	}, "")
}

func (h *HubspotApi) UpdateContactPropertyImpl(ctx context.Context, adminID int, properties []hubspot.Property) error {
	admin := &model.Admin{}
	if err := h.db.Model(&model.Admin{}).Where("id = ?", adminID).Take(&admin).Error; err != nil {
		return err
	}
	hubspotContactID := admin.HubspotContactID
	if hubspotContactID == nil {
		id, err := h.CreateContactForAdminImpl(
			ctx,
			adminID,
			ptr.ToString(admin.Email),
			ptr.ToString(admin.UserDefinedRole),
			ptr.ToString(admin.UserDefinedPersona),
			ptr.ToString(admin.FirstName),
			ptr.ToString(admin.LastName),
			ptr.ToString(admin.Phone),
			ptr.ToString(admin.Referral),
		)
		if err != nil {
			return err
		}
		hubspotContactID = id
	}
	if err := h.hubspotClient.Contacts().Update(ptr.ToInt(hubspotContactID), hubspot.ContactsRequest{
		Properties: properties,
	}); err != nil {
		return err
	}
	return nil
}

func (h *HubspotApi) UpdateCompanyProperty(ctx context.Context, workspaceID int, properties []hubspot.Property) error {
	return h.kafkaProducer.Submit(ctx, &kafka_queue.Message{
		Type: kafka_queue.HubSpotUpdateCompanyProperty,
		HubSpotUpdateCompanyProperty: &kafka_queue.HubSpotUpdateCompanyPropertyArgs{
			WorkspaceID: workspaceID,
			Properties:  properties,
		},
	}, "")
}

func (h *HubspotApi) UpdateCompanyPropertyImpl(ctx context.Context, workspaceID int, properties []hubspot.Property) error {
	workspace := &model.Workspace{}
	if err := h.db.Model(&model.Workspace{}).Where("id = ?", workspaceID).Take(&workspace).Error; err != nil {
		return err
	}
	hubspotWorkspaceID := workspace.HubspotCompanyID
	if hubspotWorkspaceID == nil && workspace.ID != 0 {
		id, err := h.CreateCompanyForWorkspaceImpl(
			ctx,
			workspaceID,
			"", ptr.ToString(workspace.Name),
		)
		if err != nil {
			return err
		}
		hubspotWorkspaceID = id
	}
	if _, err := h.hubspotClient.Companies().Update(ptr.ToInt(hubspotWorkspaceID), hubspot.CompaniesRequest{
		Properties: properties,
	}); err != nil {
		return err
	}
	return nil
}
