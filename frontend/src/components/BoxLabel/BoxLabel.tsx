import { Stack, Text } from '@highlight-run/ui'
import React from 'react'

interface BoxLabelProps {
	label?: string
	info?: string | React.ReactNode
}

const BoxLabel = ({ label, info }: BoxLabelProps) => {
	return (
		<Stack gap="12" direction="column" my="6">
			{label && (
				<Text weight="bold" size="small" color="strong">
					{label}
				</Text>
			)}
			{info && <Text color="moderate">{info}</Text>}
		</Stack>
	)
}

export default BoxLabel
