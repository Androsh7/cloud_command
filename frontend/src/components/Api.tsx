import axios, { type AxiosRequestConfig } from "axios";
import {
  AgentModelSchema,
  AgentStatusModelSchema,
  CommandResultModelSchema,
  FileUploadModelSchema,
  ShellSessionModelSchema,
  type CommandModel,
  type AgentModel,
  type AgentStatusModel,
  type CommandResultModel,
  type FileUploadModel,
  type ShellSessionModel,
} from "./Models";
import { z, ZodType } from "zod";

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
  return api.delete<void>(`/agent/${name}/delete`);
}

export function getAgentState(name: string): Promise<string> {
  return axiosZod(z.string(), {
    method: "GET",
    url: `/agent/${name}/state`,
  });
}

export function executeCommand(
  agentName: string,
  command: string,
  options?: Pick<CommandModel, "executable" | "sudo">,
): Promise<CommandResultModel> {
  return axiosZod(CommandResultModelSchema, {
    method: "POST",
    url: `/agent/${agentName}/command/run_command`,
    data: {
      command,
      ...options,
    },
  });
}

export function statusAgent(agentName: string): Promise<AgentStatusModel> {
  return axiosZod(AgentStatusModelSchema, {
    method: "GET",
    url: `/agent/${agentName}/command/statistics`,
  });
}

export function uploadAgentFile(
  agentName: string,
  destinationPath: string,
  file: File,
): Promise<FileUploadModel> {
  const formData = new FormData();
  formData.append("file", file);

  return axiosZod(FileUploadModelSchema, {
    method: "POST",
    url: `/agent/${agentName}/command/upload`,
    params: { path: destinationPath },
    data: formData,
    headers: {
      "Content-Type": "multipart/form-data",
    },
  });
}

export function downloadAgentFile(
  agentName: string,
  path: string,
): Promise<Blob> {
  return api
    .request<Blob>({
      method: "GET",
      url: `/agent/${agentName}/command/download`,
      params: { path },
      responseType: "blob",
    })
    .then((res) => res.data);
}

export function listShellSessions(): Promise<ShellSessionModel[]> {
  return axiosZod(ShellSessionModelSchema.array(), {
    method: "GET",
    url: "/shell/list",
  });
}

export function createShellSession(agent: string): Promise<string> {
  return axiosZod(z.string().uuid(), {
    method: "POST",
    url: "/shell/create",
    data: { agent },
  });
}

export function deleteShellSession(uuid: string) {
  return api.delete<void>(`/shell/${uuid}`);
}

export function sendShellCommand(uuid: string, sendCommand: string) {
  return api.request<void>({
    method: "POST",
    url: `/shell/${uuid}/send_command`,
    data: { send_command: sendCommand },
  });
}

export function getShellOutput(uuid: string): Promise<string> {
  return axiosZod(z.string(), {
    method: "GET",
    url: `/shell/${uuid}/get_output`,
  });
}

export function getShellStatistics(uuid: string): Promise<AgentStatusModel> {
  return axiosZod(AgentStatusModelSchema, {
    method: "POST",
    url: `/shell/${uuid}/get_statistics`,
  });
}
