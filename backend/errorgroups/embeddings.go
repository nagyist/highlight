package errorgroups

import (
	"context"
	"github.com/highlight-run/highlight/backend/model"
	e "github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	log "github.com/sirupsen/logrus"
	"os"
	"time"
)

type EmbeddingType string

const EventEmbedding EmbeddingType = "EventEmbedding"
const StackTraceEmbedding EmbeddingType = "StackTraceEmbedding"
const PayloadEmbedding EmbeddingType = "PayloadEmbedding"

func GetEmbeddings(ctx context.Context, errors []*model.ErrorObject) ([]*model.ErrorObjectEmbeddings, error) {
	start := time.Now()
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, e.New("OPENAI_API_KEY is not set")
	}
	client := openai.NewClient(apiKey)

	var eventErrors, stacktraceErrors, payloadErrors []*model.ErrorObject
	var eventInputs, stacktraceInputs, payloadInputs []string
	for _, errorObject := range errors {
		var stackTrace *string
		if errorObject.MappedStackTrace != nil {
			stackTrace = errorObject.MappedStackTrace
		} else {
			stackTrace = errorObject.StackTrace
		}
		eventInputs = append(eventInputs, errorObject.Event)
		eventErrors = append(eventErrors, errorObject)
		if stackTrace != nil {
			stacktraceInputs = append(stacktraceInputs, *stackTrace)
			stacktraceErrors = append(stacktraceErrors, errorObject)
		}
		if errorObject.Payload != nil {
			payloadInputs = append(payloadInputs, *errorObject.Payload)
			payloadErrors = append(payloadErrors, errorObject)
		}
	}

	results := map[int]*model.ErrorObjectEmbeddings{}
	for _, inputs := range []struct {
		inputs    []string
		errors    []*model.ErrorObject
		embedding EmbeddingType
	}{
		{inputs: eventInputs, errors: eventErrors, embedding: EventEmbedding},
		{inputs: stacktraceInputs, errors: stacktraceErrors, embedding: StackTraceEmbedding},
		{inputs: payloadInputs, errors: payloadErrors, embedding: PayloadEmbedding},
	} {
		if len(inputs.inputs) == 0 {
			continue
		}
		resp, err := client.CreateEmbeddings(
			context.Background(),
			openai.EmbeddingRequest{
				Input: inputs.inputs,
				Model: openai.AdaEmbeddingV2,
				User:  "highlight-io",
			},
		)
		if err != nil {
			return nil, err
		}
		log.WithContext(ctx).
			WithField("num_inputs", len(inputs.inputs)).
			WithField("time", time.Since(start)).
			WithField("embedding", inputs.embedding).
			Info("AI embedding generated.")

		for idx, errorObject := range inputs.errors {
			if _, ok := results[errorObject.ID]; !ok {
				results[errorObject.ID] = &model.ErrorObjectEmbeddings{ErrorObjectID: errorObject.ID}
			}
			switch inputs.embedding {
			case EventEmbedding:
				results[errorObject.ID].EventEmbedding = resp.Data[idx].Embedding
			case StackTraceEmbedding:
				results[errorObject.ID].StackTraceEmbedding = resp.Data[idx].Embedding
			case PayloadEmbedding:
				results[errorObject.ID].PayloadEmbedding = resp.Data[idx].Embedding
			}
		}
	}

	return lo.Values(results), nil
}
