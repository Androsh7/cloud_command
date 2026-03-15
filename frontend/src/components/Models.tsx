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

export const AgentModelSchema = z.object({
  name: z.string(),
  state: z.string(),
  public_ip_address: z.string().nullable().optional(),
  config: AgentConfigModelSchema,
});
export type AgentModel = z.infer<typeof AgentModelSchema>;

export const AgentStatusModelSchema = z.object({
  uptime_seconds: z.number(),
  disk_usage: z.string(),
  ram_usage: z.string(),
  cpu_usage: z.string(),
});
export type AgentStatusModel = z.infer<typeof AgentStatusModelSchema>;

export const CreateEC2AgentModelSchema = z.object({
  region: z.string(),
  instance_type: z.string(),
  architecture: z.enum(["x86_64", "arm64"]),
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

export const AgentUploadBodySchema = z.object({
  file: z.instanceof(File),
});
export type AgentUploadBody = z.infer<typeof AgentUploadBodySchema>;

export const ErrorResponseSchema = z.object({
  error: z.string(),
  details: z.string(),
});
export type ErrorResponse = z.infer<typeof ErrorResponseSchema>;

export const ValidationErrorSchema = z.object({
  loc: z.array(z.union([z.string(), z.number().int()])),
  msg: z.string(),
  type: z.string(),
  input: z.unknown().optional(),
  ctx: z.record(z.string(), z.unknown()).optional(),
});
export type ValidationError = z.infer<typeof ValidationErrorSchema>;

export const HTTPValidationErrorSchema = z.object({
  detail: z.array(ValidationErrorSchema).optional(),
});
export type HTTPValidationError = z.infer<typeof HTTPValidationErrorSchema>;

export const CreateShellSessionModelSchema = z.object({
  agent: z.string(),
});
export type CreateShellSessionModel = z.infer<
  typeof CreateShellSessionModelSchema
>;

export const ShellSessionModelSchema = z.object({
  agent_name: z.string(),
  uuid: z.string().uuid(),
});
export type ShellSessionModel = z.infer<typeof ShellSessionModelSchema>;

export const TmuxSendKeysSchema = z.object({
  send_command: z.string(),
});
export type TmuxSendKeys = z.infer<typeof TmuxSendKeysSchema>;
