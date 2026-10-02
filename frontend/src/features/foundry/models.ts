export interface Foundry {
  id: string;
  name: string;
  subscriptionName: string;
  resourceGroupName: string;
}

export interface Deployment {
  id: string;
  deploymentName: string;
  modelName: string;
  version: string;
}

export interface InitialFoundryView {
  foundries: Foundry[];
  selectedFoundryId: string;
  deployments: Deployment[];
}
