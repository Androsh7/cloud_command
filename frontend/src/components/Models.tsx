import * as z from "zod";

export const SshKeyPairModelSchema = z.object({
  private_key: z.string(),
  public_key: z.string(),
});
export type SshKeyPairModel = z.infer<typeof SshKeyPairModelSchema>;

export const AgentConfigModelSchema = z.object({
  instance_type: z.string(),
  region: z.string(),
  ami_id: z.string(),
  instance_id: z.string(),
  vpc_id: z.string(),
  subnet_id: z.string(),
  security_group_id: z.string(),
  key_pair_name: z.string(),
  key_pair: SshKeyPairModelSchema,
});
export type AgentConfigModel = z.infer<typeof AgentConfigModelSchema>;

export const AgentLocationModelSchema = z.object({
  public_ip_address: z.string(),
  hostname: z.string(),
  city: z.string(),
  region: z.string(),
  country: z.string(),
  loc: z.string(),
  org: z.string(),
  postal: z.string(),
  timezone: z.string(),
});
export type AgentLocationModel = z.infer<typeof AgentLocationModelSchema>;

export const AgentModelSchema = z.object({
  name: z.string(),
  state: z.string(),
  public_ip_address: z.string().nullable().optional(),
  config: AgentConfigModelSchema,
});
export type AgentModel = z.infer<typeof AgentModelSchema>;

export const AgentStatusModelSchema = z.object({
  uptime_seconds: z.number(),
  location: AgentLocationModelSchema,
  disk_usage: z.string(),
  ram_usage: z.string(),
  cpu_usage: z.string(),
});
export type AgentStatusModel = z.infer<typeof AgentStatusModelSchema>;

export const CreateEC2AgentModelSchema = z.object({
  region: z.string(),
  instance_type: z.string(),
});
export type CreateEC2AgentModel = z.infer<typeof CreateEC2AgentModelSchema>;

export const CommandModelSchema = z.object({
  command: z.string(),
  executable: z.string().default("/bin/bash").optional(),
  sudo: z.boolean().default(false).optional(),
});
export type CommandModel = z.infer<typeof CommandModelSchema>;

export const CommandResultModelSchema = z.object({
  stdout: z.string(),
  stderr: z.string(),
  exit_code: z.number().int(),
});
export type CommandResultModel = z.infer<typeof CommandResultModelSchema>;

export const FileUploadModelSchema = z.object({
  destination_path: z.string(),
  file_size_bytes: z.number().int(),
  mime_type: z.string(),
});
export type FileUploadModel = z.infer<typeof FileUploadModelSchema>;
