import { useEffect, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import {
	Badge,
	Box,
	ButtonIcon,
	Heading,
	IconSolidBookOpen,
	IconSolidClipboard,
	IconSolidDiscord,
	IconSolidExternalLink,
	IconSolidPencilAlt,
	Stack,
	Tag,
	Text,
} from '@highlight-run/ui/components'
import { quickStartContentReorganized, QuickStartContent } from 'highlight.io'

import { useProjectId } from '@/hooks/useProjectId'
import { useGetProjectQuery } from '@/graph/generated/hooks'
import LoadingBox from '@/components/LoadingBox'
import analytics from '@/util/analytics'
import { copyToClipboard } from '@/util/string'
import { vars } from '@highlight-run/ui/vars'
import { LinkButton } from '@/components/LinkButton'
import { useParams } from '@util/react-router/useParams'

import { ICON_MAPPINGS } from '../constants'
import { QuickStartGuide } from '../QuickStartGuide'
import { FeatureHealthCheck } from '../FeatureHealthCheck'

const ICON_FILL = vars.theme.interactive.fill.secondary.content.text

export const ConnectPage = () => {
	const { projectId } = useProjectId()
	const navigate = useNavigate()
	const { language, platform } = useParams<{
		language: string
		platform: string
	}>()

	const { data } = useGetProjectQuery({
		variables: { id: projectId! },
		fetchPolicy: 'no-cache',
	})
	const projectVerboseId = data?.project?.verbose_id
	const platforms = data?.project?.platforms

	useEffect(() => {
		if (data?.project && !data?.project?.platforms?.length) {
			navigate(`/${projectId}/connect/new`)
		}
	}, [data, navigate, projectId])

	useEffect(() => analytics.page('Connect'), [])

	const guide = useMemo(() => {
		// return selected platform if valid
		if (language && platform) {
			const sdk = (quickStartContentReorganized as any)[language]?.sdks[
				platform
			] as QuickStartContent
			if (sdk) {
				return sdk
			}
		}

		// If no valid selected platform, find first valid platform
		const selectedSdks = data?.project?.platforms?.map((platform) => {
			const [sdkLanguage, sdkPlatform] = platform.split('_')
			return (quickStartContentReorganized as any)[sdkLanguage]?.sdks[
				sdkPlatform
			] as QuickStartContent
		})

		return selectedSdks?.find((sdk) => sdk)
	}, [language, platform, data])

	if (!projectVerboseId || !platforms?.length) {
		return <LoadingBox />
	}

	const copyProjectId = () => {
		copyToClipboard(projectVerboseId!, { onCopyText: 'Copied project ID!' })
	}

	return (
		<Box
			background="n2"
			padding="8"
			flex="stretch"
			justifyContent="stretch"
			display="flex"
		>
			<Box
				background="white"
				borderRadius="6"
				flexDirection="column"
				display="flex"
				flexGrow={1}
				p="48"
				border="dividerWeak"
				overflowY="auto"
				shadow="medium"
			>
				<Stack
					gap="40"
					mx="auto"
					style={{ maxWidth: 960 }}
					width="full"
				>
					<Heading level="h2">Connect Platforms</Heading>
					<Stack direction="row" gap="32">
						<Stack gap="12" flexGrow={0} style={{ maxWidth: 310 }}>
							<Stack pt="2" gap="8">
								<Text color="moderate">Platforms</Text>
								<SelectedPlatformButtons
									projectId={projectId!}
									platforms={data?.project?.platforms || []}
								/>
								<LinkButton
									trackingId="edit_platforms"
									kind="secondary"
									emphasis="medium"
									iconLeft={<IconSolidPencilAlt />}
									to={`/${projectId}/connect/new`}
								>
									Edit platforms
								</LinkButton>
							</Stack>
							<Box as="span" borderBottom="divider" />
							<Stack
								direction="row"
								border="dividerWeak"
								borderRadius="6"
								justifyContent="space-between"
								px="4"
								background="raised"
							>
								<Stack direction="row" gap="6" align="center">
									<Text color="moderate" size="xSmall">
										Project ID:
									</Text>
									<Tag
										kind="secondary"
										emphasis="low"
										shape="basic"
										onClick={copyProjectId}
									>
										{projectVerboseId}
									</Tag>
								</Stack>
								<ButtonIcon
									kind="secondary"
									emphasis="low"
									icon={<IconSolidClipboard />}
									onClick={copyProjectId}
								/>
							</Stack>
							<Text color="moderate">
								Don't see your platform? Let us know in the
								Discord channel.
							</Text>
							<Box display="flex" gap="8">
								<a
									href="https://discord.gg/yxaXEAqgwN"
									target="_blank"
									rel="noreferrer"
									style={{ display: 'flex' }}
								>
									<Badge
										variant="outlineGray"
										label="Highlight.io"
										size="medium"
										iconStart={
											<IconSolidDiscord
												fill={ICON_FILL}
											/>
										}
										iconEnd={
											<IconSolidExternalLink
												fill={ICON_FILL}
											/>
										}
									/>
								</a>
								<a
									href="https://www.highlight.io/docs/getting-started/overview"
									target="_blank"
									rel="noreferrer"
									style={{ display: 'flex' }}
								>
									<Badge
										variant="outlineGray"
										label="Full Documentation"
										size="medium"
										iconStart={
											<IconSolidBookOpen
												fill={ICON_FILL}
											/>
										}
										iconEnd={
											<IconSolidExternalLink
												fill={ICON_FILL}
											/>
										}
									>
										Highlight.io
									</Badge>
								</a>
							</Box>
						</Stack>
						<Stack gap="24" flexGrow={1} style={{ maxWidth: 650 }}>
							<FeatureHealthCheck />
							<QuickStartGuide
								guide={guide}
								projectVerboseId={projectVerboseId}
							/>
						</Stack>
					</Stack>
				</Stack>
			</Box>
		</Box>
	)
}

type SelectedPlatformIconsProps = {
	projectId: string
	platforms: string[]
}

const SelectedPlatformButtons = ({
	projectId,
	platforms,
}: SelectedPlatformIconsProps) => {
	const navigate = useNavigate()
	const { language, platform } = useParams<{
		language: string
		platform: string
	}>()

	if (platforms.length <= 0) {
		return <Text color="moderate">None</Text>
	}

	const redirectToDoc = (language: string, platform: string) => {
		navigate(`/${projectId}/connect/${language}/${platform}`)
	}

	return (
		<Stack gap="8">
			{platforms.map((identifier, index) => {
				const [sdkLanguage, sdkPlatform] = identifier.split('_')
				const sdk = (quickStartContentReorganized as any)[sdkLanguage]
					?.sdks[sdkPlatform] as QuickStartContent

				if (!sdk) {
					console.warn(`Invalid selected platform: ${identifier}`)
					return null
				}

				const selected =
					!language || !platform
						? index === 0
						: language === sdkLanguage && platform === sdkPlatform

				return (
					<Stack
						key={identifier}
						align="center"
						direction="row"
						justifyContent="space-between"
						background={selected ? 'secondarySelected' : 'default'}
						borderRadius="8"
						py="8"
						px="8"
						gap="10"
						cursor="pointer"
						as="button"
						border="dividerWeak"
						borderWidth="medium"
						onClick={() => redirectToDoc(sdkLanguage, sdkPlatform)}
					>
						<Stack direction="row" align="center" gap="10">
							<Box
								alignItems="center"
								display="flex"
								justifyContent="center"
								borderRadius="5"
								border="dividerWeak"
								borderWidth="medium"
							>
								{sdk.logoKey &&
								ICON_MAPPINGS.hasOwnProperty(sdk.logoKey) ? (
									<img
										alt={sdk.title}
										title={sdk.title}
										src={
											(ICON_MAPPINGS as any)[sdk.logoKey]
										}
										style={{
											height: 30,
											width: 30,
											borderRadius: 5,
										}}
									/>
								) : (
									<Text
										userSelect="none"
										weight="bold"
										title={sdk.title}
									>
										{sdk.title[0].toUpperCase()}
									</Text>
								)}
							</Box>
							<Text weight="bold">{sdk.title}</Text>
						</Stack>
						<Badge
							label="Docs"
							variant="outlineGray"
							iconStart={<IconSolidBookOpen />}
						/>
					</Stack>
				)
			})}
		</Stack>
	)
}
