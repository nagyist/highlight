import { siteUrl } from '../../../../utils/urls'
import { QuickStartContent } from '../../QuickstartContent'
import { verifyLogs } from '../shared-snippets'

export const HostingFlyIOLogContent: QuickStartContent = {
	title: 'Logging with the Fly.IO Log Shipper',
	subtitle:
		'Learn how to setup Highlight log ingestion on [Fly.io](https://fly.io/blog/shipping-logs/). ' +
		'As a prerequisite, we assume you already have an application ' +
		'deployed on Fly.io and `flyctl` configured locally.',
	logoUrl: siteUrl('/images/quickstart/fly-io.svg'),
	entries: [
		{
			title: 'Spin up the highlight.io logs shipper alongside your fly.io deployment.',
			content:
				'Clone our sample fly-logs-shipper fork that configures the highlight-io vector logs drain. ' +
				'Launching an empty app first is a workaround until our logs shipper is merged with the main one.',
			code: [
				{
					text: `git clone git@github.com:highlight/fly-log-shipper.git`,
					language: 'bash',
				},
				{
					text: `cd fly-log-shipper`,
					language: 'bash',
				},
				{
					text: `mv fly.toml fly.toml.temp`,
					language: 'bash',
				},
				{
					text: `fly launch`,
					language: 'bash',
				},
				{
					text: `mv fly.toml.temp fly.toml`,
					language: 'bash',
				},
			],
		},
		{
			title: 'Configure and launch the logs shipper.',
			content:
				'No other work is needed on the side of your application, ' +
				'as fly apps are already sending monitoring information ' +
				'back to fly which we can read. ' +
				'Check out the `README.md` for more details.',
			code: [
				{
					text: `# set the org for your deployment
fly secrets set ORG=personal`,
					language: 'bash',
				},
				{
					text: `# give the logs shipper access to other containers' logs
fly secrets set ACCESS_TOKEN=$(fly auth token)`,
					language: 'bash',
				},
				{
					text: `# set to configure your highlight project
fly secrets set HIGHLIGHT_PROJECT_ID=<YOUR_PROJECT_ID>`,
					language: 'bash',
				},
				{
					text: `fly launch`,
					language: 'bash',
				},
			],
		},
		verifyLogs,
	],
}
