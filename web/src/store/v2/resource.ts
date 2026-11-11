import { makeAutoObservable } from "mobx";
import { resourceServiceClient } from "@/grpcweb";
import { CreateResourceRequest, Resource, UpdateResourceRequest } from "@/types/proto/api/v1/resource_service";

class LocalState {
  resourceMapByName: Record<string, Resource> = {};
  inflightFetchByName: Record<string, Promise<Resource>> = {};

  constructor() {
    makeAutoObservable(this);
  }

  setPartial(partial: Partial<LocalState>) {
    Object.assign(this, partial);
  }
}

const resourceStore = (() => {
  const state = new LocalState();

  const fetchResourceByName = async (name: string, options?: { skipCache?: boolean }) => {
    if (!options?.skipCache) {
      const cached = state.resourceMapByName[name];
      if (cached) {
        return cached;
      }
    }
    const inflight = state.inflightFetchByName[name];
    if (inflight) {
      return inflight;
    }

    const request = resourceServiceClient
      .getResource({
        name,
      })
      .then((resource) => {
        const resourceMap = { ...state.resourceMapByName };
        resourceMap[resource.name] = resource;
        state.setPartial({ resourceMapByName: resourceMap });
        return resource;
      })
      .finally(() => {
        const inflightMap = { ...state.inflightFetchByName };
        delete inflightMap[name];
        state.setPartial({ inflightFetchByName: inflightMap });
      });

    state.setPartial({
      inflightFetchByName: {
        ...state.inflightFetchByName,
        [name]: request,
      },
    });

    return request;
  };

  const getResourceByName = (name: string) => {
    return state.resourceMapByName[name];
  };

  const createResource = async (create: CreateResourceRequest): Promise<Resource> => {
    const resource = await resourceServiceClient.createResource(create);
    const resourceMap = { ...state.resourceMapByName };
    resourceMap[resource.name] = resource;
    state.setPartial({ resourceMapByName: resourceMap });
    return resource;
  };

  const updateResource = async (update: UpdateResourceRequest): Promise<Resource> => {
    const resource = await resourceServiceClient.updateResource(update);
    const resourceMap = { ...state.resourceMapByName };
    resourceMap[resource.name] = resource;
    state.setPartial({ resourceMapByName: resourceMap });
    return resource;
  };

  return {
    state,
    fetchResourceByName,
    getResourceByName,
    createResource,
    updateResource,
  };
})();

export default resourceStore;
