import { createFileRoute } from '@tanstack/react-router';
import { AzureLogin } from '../usecases/azure-login/AzureLogin';
export const Route = createFileRoute('/')({ component: AzureLogin });
