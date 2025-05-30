import { Integration } from '../Integrations/integration'

export const FRAMEWORKS: {
	[key: string]: Integration[]
} = {
	Frameworks: [
		{
			name: 'React.js',
			description: 'Set up highlight.io with your React application.',
			link: '/docs/getting-started/browser/reactjs',
			image: '/images/companies/icons/react.svg',
		},
		{
			name: 'Next.js',
			description: 'Set up highlight.io with your Next application.',
			link: '/docs/getting-started/browser/nextjs',
			image: '/images/companies/icons/nextjs.svg',
		},
		{
			name: 'Remix',
			description: 'Set up highlight.io with your Remix application.',
			link: '/docs/getting-started/browser/remix',
			image: '/images/companies/icons/remix.png',
		},
		{
			name: 'Vue.js',
			description: 'Set up highlight.io with your Vue application.',
			link: '/docs/getting-started/browser/vuejs',
			image: '/images/companies/icons/vuejs.svg',
		},
		{
			name: 'Angular',
			description: 'Set up highlight.io with your Angular application.',
			link: '/docs/getting-started/browser/angular',
			image: '/images/companies/icons/angularjs.svg',
		},
		{
			name: 'Gatsby.js',
			description: 'Set up highlight.io with your Gatsby application.',
			link: '/docs/getting-started/browser/gatsbyjs',
			image: '/images/companies/icons/gatsby.svg',
		},
		{
			name: 'SvelteKit',
			description: 'Set up highlight.io with your SvelteKit application.',
			link: '/docs/getting-started/browser/sveltekit',
			image: '/images/companies/icons/sveltekit.svg',
		},
	],
	'Error Monitoring': [
		{
			name: 'Go',
			description:
				'Set up error monitoring in Go with Chi, Fiber, and more.',
			link: '/docs/getting-started/server/go/overview',
			image: '/images/companies/icons/go.svg',
		},
		{
			name: 'Javascript',
			description:
				'Set up error monitoring in JS with Express, Node, and more.',
			link: '/docs/getting-started/server/js/overview',
			image: '/images/companies/icons/js.svg',
		},
		{
			name: 'Python',
			description:
				'Set up error monitoring in Python with Django, Flask, and more.',
			link: '/docs/getting-started/server/python/overview',
			image: '/images/companies/icons/python.svg',
		},
		{
			name: 'Ruby',
			description: 'Set up error monitoring in Ruby on Rails.',
			link: '/docs/getting-started/server/ruby/overview',
			image: '/images/companies/icons/rails.svg',
		},
		{
			name: 'Rust',
			description: 'Set up error monitoring in Rust.',
			link: '/docs/getting-started/server/rust/overview',
			image: '/images/companies/icons/rust.svg',
		},
	],
	Logging: [
		{
			name: 'Go',
			description: 'Set up logging in Go with Fiber, Logrus, and more.',
			link: '/docs/getting-started/server/go/overview',
			image: '/images/companies/icons/go.svg',
		},
		{
			name: 'Javascript',
			description: 'Set up logging in JS with Nest, Winston, and more.',
			link: '/docs/getting-started/server/js/overview',
			image: '/images/companies/icons/js.svg',
		},
		{
			name: 'Python',
			description: 'Set up logging in Python with Loguru.',
			link: '/docs/getting-started/server/python/overview',
			image: '/images/companies/icons/python.svg',
		},
		{
			name: 'Ruby',
			description: 'Set up logging in Ruby on Rails.',
			link: '/docs/getting-started/server/ruby/overview',
			image: '/images/companies/icons/rails.svg',
		},
		{
			name: 'Rust',
			description: 'Set up logging in Rust.',
			link: '/docs/getting-started/server/rust/overview',
			image: '/images/companies/icons/rust.svg',
		},
	],
	Protocols: [
		{
			name: 'cURL',
			description: 'Set up highlight.io log ingestion over HTTPS.',
			link: '/docs/getting-started/server/http',
			image: '/images/companies/icons/curl.svg',
		},
		{
			name: 'File',
			description:
				'Set up log ingestion using an OpenTelemetry collector with the filelog receiver.',
			link: '/docs/getting-started/server/file',
			image: '/images/companies/icons/file.svg',
		},
		{
			name: 'Fluent Forward',
			description:
				'Set up highlight.io log ingestion via Fluent Forward.',
			link: '/docs/getting-started/server/fluentforward',
			image: '/images/companies/icons/fluent.svg',
		},
	],
}
