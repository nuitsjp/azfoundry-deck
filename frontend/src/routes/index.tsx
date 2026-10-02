import { createFileRoute } from '@tanstack/react-router';
import { Title } from '@mantine/core';
export const Route = createFileRoute('/')({ component: () => <Title order={2}>Home</Title> });
