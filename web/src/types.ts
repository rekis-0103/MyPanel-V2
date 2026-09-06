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
  ownerUserId: string;
  ownerUsername: string;
  subscriptionStatus?: Subscription['status'] | null;
  subscriptionEndsAt?: string | null;
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

export type MinecraftVersion = { id: string; java: 21 | 25 };
export type Runtime = { id: string; name: string; java: number; javaVersions: Array<21 | 25>; icon?: 'grass' | 'feather' | 'crystal' | 'anvil'; versions?: MinecraftVersion[] };
export type Session = { userId: string; username: string; role: 'owner' | 'user'; mustChangePassword: boolean; csrfToken: string };

export type HostingPackage = { id: string; slug: string; name: string; description: string; priceIdr: number; cpu: number; memoryMb: number; diskMb: number; sortOrder: number; active: boolean; createdAt: string; updatedAt: string };
export type CapacityNumbers = { cpu: number; memoryMb: number; diskMb: number; ports: number };
export type Capacity = { total?: CapacityNumbers; reserved?: CapacityNumbers; available: CapacityNumbers; packageAvailability: Record<string, boolean>; host?: { cpu: number; memoryBytes: number; memoryAvailableBytes: number; diskBytes: number; diskAvailableBytes: number; uptimeSeconds: number }; hostUnavailable?: boolean };
export type Order = { id: string; userId: string; serverId: string; subscriptionId: string; packageId: string | null; kind: 'purchase' | 'renewal'; status: 'paid' | 'action_required'; amountIdr: number; packageName: string; cpu: number; memoryMb: number; diskMb: number; paymentReference: string; periodStart: string; periodEnd: string; createdAt: string };
export type Subscription = { id: string; userId: string; serverId: string; packageId: string | null; packageName: string; priceIdr: number; cpu: number; memoryMb: number; diskMb: number; status: 'provisioning' | 'active' | 'action_required' | 'grace' | 'released' | 'canceled'; currentPeriodStart: string; currentPeriodEnd: string; graceEndsAt: string; resourceReleasedAt: string | null; createdAt: string; updatedAt: string };
export type PanelUser = { id: string; username: string; role: 'owner' | 'user'; status: 'active' | 'suspended'; mustChangePassword: boolean; createdAt: string; updatedAt: string };
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
