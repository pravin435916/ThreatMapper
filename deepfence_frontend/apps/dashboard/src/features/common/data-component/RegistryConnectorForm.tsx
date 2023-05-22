import { useEffect } from 'react';
import { ActionFunctionArgs, useFetcher } from 'react-router-dom';
import { toast } from 'sonner';

import { getRegistriesApiClient } from '@/api/api';
import { ApiDocsBadRequestResponse, ModelRegistryAddReq } from '@/api/generated';
import { AmazonECRConnectorForm } from '@/components/registries-connector/AmazonECRConnectorForm';
import { AzureCRConnectorForm } from '@/components/registries-connector/AzureCRConnectorForm';
import { DockerConnectorForm as DockerRegistryConnectorForm } from '@/components/registries-connector/DockerConnectorForm';
import { DockerPriavateConnectorForm } from '@/components/registries-connector/DockerPrivateConnectorForm';
import { GitLabConnectorForm } from '@/components/registries-connector/GitLabConnectorForm';
import { GoogleCRConnectorForm } from '@/components/registries-connector/GoogleCRConnectorForm';
import { HarborConnectorForm } from '@/components/registries-connector/HarborConnectorForm';
import { JfrogConnectorForm } from '@/components/registries-connector/JfrogConnectorForm';
import { QuayConnectorForm } from '@/components/registries-connector/QuayConnectorForm';
import { RegistryType } from '@/types/common';
import { apiWrapper } from '@/utils/api';

type ActionReturnType = {
  message?: string;
  success: boolean;
};

type FormProps = {
  onSuccess: () => void;
  renderButton: (state: 'submitting' | 'idle' | 'loading') => JSX.Element;
  registryType: string;
};

function unwrapNestedStruct(
  obj: Record<string, string>,
): Record<string, string | Record<string, string>> {
  const results: Record<string, string | Record<string, string>> = {};
  for (const key in obj) {
    if (key.includes('.')) {
      const [first, second] = key.split('.');
      if (!results[first]) {
        results[first] = {};
      }
      (results[first] as Record<string, string>)[second] = obj[key];
    }
  }

  return results;
}

const getRequestBodyByRegistryType = (
  registryType: string,
  body: {
    [k: string]: string;
  },
): ModelRegistryAddReq => {
  if (registryType in RegistryType) {
    const requestParams: ModelRegistryAddReq = {
      name: body.name,
      registry_type: registryType,
      ...unwrapNestedStruct(body),
    };
    return requestParams;
  } else {
    throw new Error('Invalid registry type');
  }
};

export const registryConnectorActionApi = async ({
  request,
}: ActionFunctionArgs): Promise<ActionReturnType> => {
  const formData = await request.formData();
  const registryType = formData.get('registryType')?.toString();

  if (!registryType) {
    throw new Error('Registry Type is required');
  }

  const formBody = Object.fromEntries(formData) as {
    [k: string]: string;
  };

  if (registryType === RegistryType.google_container_registry) {
    const addRegistryGCR = apiWrapper({ fn: getRegistriesApiClient().addRegistryGCR });

    const response = await addRegistryGCR({
      name: formBody.name.toString(),
      registryUrl: formBody.registry_url.toString(),
      serviceAccountJson: formData.get('service_account_json') as Blob,
    });

    if (!response.ok) {
      if (response.error.response.status === 400) {
        const modelResponse: ApiDocsBadRequestResponse =
          await response.error.response.json();
        return {
          success: false,
          message: modelResponse.message ?? '',
        };
      }
    }
  } else {
    const addRegistry = apiWrapper({ fn: getRegistriesApiClient().addRegistry });

    const response = await addRegistry({
      modelRegistryAddReq: getRequestBodyByRegistryType(registryType, formBody),
    });

    if (!response.ok) {
      if (response.error.response.status === 400) {
        const modelResponse: ApiDocsBadRequestResponse =
          await response.error.response.json();
        return {
          success: false,
          message: modelResponse.message ?? '',
        };
      }
    }
  }

  toast('Registry added successfully');
  return {
    success: true,
  };
};

export const RegistryConnectorForm = ({
  onSuccess,
  renderButton,
  registryType,
}: FormProps) => {
  const fetcher = useFetcher<ActionReturnType>();

  useEffect(() => {
    if (fetcher?.data?.success) {
      onSuccess();
    }
  }, [fetcher]);

  return (
    <fetcher.Form
      method="post"
      action={'/data-component/registries/add-connector'}
      encType="multipart/form-data"
    >
      {registryType === RegistryType.docker_hub && (
        <DockerRegistryConnectorForm errorMessage={fetcher?.data?.message ?? ''} />
      )}

      {registryType === RegistryType.ecr && <AmazonECRConnectorForm />}
      {registryType === RegistryType.azure_container_registry && <AzureCRConnectorForm />}
      {registryType === RegistryType.google_container_registry && (
        <GoogleCRConnectorForm />
      )}

      {registryType === RegistryType.docker_private_registry && (
        <DockerPriavateConnectorForm />
      )}

      {registryType === RegistryType.harbor && <HarborConnectorForm />}

      {registryType === RegistryType.gitlab && <GitLabConnectorForm />}

      {registryType === RegistryType.jfrog_container_registry && <JfrogConnectorForm />}

      {registryType === RegistryType.quay && <QuayConnectorForm />}

      <input type="text" name="registryType" hidden readOnly value={registryType} />
      {renderButton(fetcher.state)}
    </fetcher.Form>
  );
};
