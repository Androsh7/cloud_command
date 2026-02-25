import axios, { type AxiosRequestConfig } from "axios";
import {
  AgentModelSchema,
  CommandModelSchema,
  CreateEC2AgentModelSchema,
  type AgentModel,
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

export function createAgent(region: string, instanceType: string) {
  return axiosZod(CreateEC2AgentModelSchema, {
    method: "POST",
    url: "/agents",
    data: {
      region,
      instance_type: instanceType,
    },
  });
}

export function deleteAgent(name: string) {
  return api.delete(`/agents/${name}`);
}

export function executeCommand(agentName: string, command: string) {
  return axiosZod(CommandModelSchema, {
    method: "POST",
    url: `/agents/${agentName}/execute_command`,
    data: {
      command,
    },
  });
}
