export type JobStatus = 'queued' | 'running' | 'completed' | 'failed' | 'canceled';
export type ServerState = 'installing' | 'starting' | 'running' | 'stopping' | 'offline' | 'error' | 'deleting';

export type Job = {
  id: string;
  serverId: string;
  action: string;
  status: JobStatus;
  error: string | null;
  createdAt: string;
};

export type Server = {
  id: string;
  nodeId: string;
  name: string;
  runtime: string;
  version: string;
  javaVersion: 21 | 25;
  memoryMb: number;
  cpu: number;
  diskMb: number;
  bindIp: string;
  port: number;
  desiredState: string;
  state: ServerState;
  config: Record<string, unknown>;
  lastError: string | null;
  createdAt: string;
  updatedAt: string;
  currentJob?: Job;
};

export type Backup = {
  id: string;
  serverId: string;
  name: string;
  sizeBytes: number;
  checksumSha256: string | null;
  status: 'queued' | 'creating' | 'ready' | 'restoring' | 'failed';
  error: string | null;
  createdAt: string;
};

export type Schedule = {
  id: string;
  serverId: string;
  name: string;
  action: string;
  payload: Record<string, unknown>;
  intervalMinutes: number;
  enabled: boolean;
  nextRunAt: string;
  lastRunAt: string | null;
};

export type FileEntry = {
  name: string;
  path: string;
  type: 'file' | 'directory' | 'symlink';
  sizeBytes: number;
  modified: string;
};

export type FileResponse =
  | { type: 'directory'; path: string; entries: FileEntry[] }
  | { type: 'file'; path: string; content: string; encoding: 'base64'; sizeBytes: number };

export type Metrics = {
  state: ServerState;
  cpuPercent: number;
  memoryBytes: number;
  diskBytes: number;
  players: number;
};

export type Runtime = { id: string; name: string; java: number; javaVersions: Array<21 | 25> };
export type Session = { username: string; role: string; csrfToken: string };
export type ActionResponse = { server: Server; job: Job };

export type AuditEvent = {
  id: number;
  username: string | null;
  action: string;
  targetType: string | null;
  targetId: string | null;
  ip: string | null;
  detail: Record<string, unknown>;
  createdAt: string;
};
