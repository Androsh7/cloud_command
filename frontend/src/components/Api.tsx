import axios, { type AxiosRequestConfig } from "axios";
import {
  AgentModelSchema,
  CommandResultModelSchema,
  type AgentModel,
  type CommandResultModel,
} from "./Models";
import { ZodType } from "zod";

const api = axios.create({
  baseURL: "http://127.0.0.1:8080/api",
});

async function axiosZod<T>(
  schema: ZodType<T>,
  config: AxiosRequestConfig,
): Promise<T> {
  const res = await api.request<unknown>(config);
  return schema.parse(res.data);
}

export function getAgents(): Promise<AgentModel[]> {
  return axiosZod(AgentModelSchema.array(), {
    method: "GET",
    url: "/agent/list",
  });
}

export function createAgent(
  name: string,
  region: string,
  instanceType: string,
) {
  return api.request<void>({
    method: "POST",
    url: `/agent/${name}/create`,
    data: {
      region,
      instance_type: instanceType,
    },
  });
}

export function deleteAgent(name: string) {
  return api.delete(`/agent/${name}/delete`);
}

export function executeCommand(
  agentName: string,
  command: string,
): Promise<CommandResultModel> {
  return axiosZod(CommandResultModelSchema, {
    method: "POST",
    url: `/agent/${agentName}/command/run`,
    data: {
      command,
    },
  });
}
