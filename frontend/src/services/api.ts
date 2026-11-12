export type OperatingMode = 'unconfigured' | 'parent' | 'child';

export interface FamilyConfig {
  mode: OperatingMode;
  familyName: string;
  parentContact: string;
  localDeviceId: string;
  localDeviceName: string;
  localChildId: string;
  localEd25519Pub: string;
  tlsFingerprint: string;
  listenPort: number;
  discoveryPort: number;
  parentEndpoint: string;
  parentCertFingerprint: string;
  parentEd25519Pub: string;
  retentionDays: number;
  setupCompleted: boolean;
  updatedAt: string;
}

export interface ChildProfile {
  id: string;
  name: string;
  avatarColor: string;
  ageGroup: string;
  deviceCount: number;
  createdAt: string;
}

export interface Device {
  id: string;
  childId: string;
  childName: string;
  name: string;
  hostname: string;
  osVersion: string;
  ipAddress: string;
  port: number;
  certFingerprint: string;
  ed25519PubKey: string;
  status: 'online' | 'offline' | 'revoked' | 'paused';
  screenLocked: boolean;
  manualPause: boolean;
  remainingMinutes: number;
  usedTodayMinutes: number;
  activeApp: string;
  dnsProtected: boolean;
  agentVersion: string;
  lastSeen: string;
  pairedAt: string;
}

export interface DiscoveryBeacon {
  protocol: string;
  deviceId: string;
  deviceName: string;
  hostname: string;
  osVersion: string;
  mode: string;
  ipAddress: string;
  tlsPort: number;
  certFingerprint: string;
  paired: boolean;
  lastSeen: string;
}

export interface AppUsageEntry {
  deviceId: string;
  dateStr: string;
  executableName: string;
  displayName: string;
  category: string;
  totalSeconds: number;
  totalMinutes: number;
  launchCount: number;
  updatedAt: string;
}

export interface ActivityEvent {
  id: number;
  deviceId: string;
  eventType: string;
  appName: string;
  executableName: string;
  windowTitle: string;
  durationSeconds: number;
  occurredAt: string;
  synced: boolean;
}

export interface DailySummary {
  deviceId: string;
  dateStr: string;
  totalSeconds: number;
  totalMinutes: number;
  blockedAttempts: number;
  loginCount: number;
  topApps: AppUsageEntry[];
}

export interface DayWindow {
  allowedStart: string;
  allowedEnd: string;
  enabled: boolean;
}

export interface ScreenTimePolicy {
  deviceId: string;
  dailyLimitMinutes: number;
  weekendLimitMinutes: number;
  bedtimeStart: string;
  bedtimeEnd: string;
  weeklySchedule: Record<string, DayWindow>;
  breakIntervalMinutes: number;
  breakDurationMinutes: number;
  manualPause: boolean;
  screenLocked: boolean;
  bonusMinutesToday: number;
  bonusDate: string;
  updatedAt: string;
}

export interface ScreenTimeEvaluation {
  deviceId: string;
  allowedNow: boolean;
  blockReason: string;
  effectiveLimitMinutes: number;
  usedTodayMinutes: number;
  remainingMinutes: number;
  inBedtime: boolean;
  inAllowedWindow: boolean;
  manualPause: boolean;
  screenLocked: boolean;
  breakReminderDue: boolean;
  nextTransitionLabel: string;
}

export interface InstalledApp {
  id: string;
  deviceId: string;
  executableName: string;
  displayName: string;
  version: string;
  publisher: string;
  installPath: string;
  firstSeenAt: string;
  lastUsedAt: string;
  newlyInstalled: boolean;
  ruleStatus: string;
}

export interface AppRule {
  id: string;
  deviceId: string;
  executableName: string;
  displayName: string;
  publisher: string;
  installPath: string;
  status: 'allowed' | 'blocked' | 'scheduled';
  dailyLimitMinutes: number;
  allowedHours: string;
  isNewlyInstalled: boolean;
  usedTodayMinutes: number;
  updatedAt: string;
}

export interface WebsitePolicy {
  deviceId: string;
  dnsMode: 'cloudflare_family' | 'cloudflare_malware' | 'custom' | 'off';
  blockAdult: boolean;
  blockMalware: boolean;
  primaryIpv4: string;
  secondaryIpv4: string;
  primaryIpv6: string;
  secondaryIpv6: string;
  allowlist: string[];
  blocklist: string[];
  transparencyNote: string;
  updatedAt: string;
}

export interface DNSVerificationStatus {
  activeDnsServers: string[];
  cloudflareFamilyActive: boolean;
  cloudflareMalwareActive: boolean;
  hostsFileSynced: boolean;
  driftDetected: boolean;
  statusSummary: string;
}

export interface BlockedDomainEvent {
  id: number;
  deviceId: string;
  domain: string;
  category: string;
  reason: string;
  occurredAt: string;
}

export interface SecurityEvent {
  id: number;
  deviceId: string;
  severity: 'info' | 'warning' | 'critical';
  category: string;
  message: string;
  details: string;
  occurredAt: string;
}

export interface ProtectionStatus {
  deviceId: string;
  dnsProtected: boolean;
  dnsMode: string;
  activeDnsServers: string[];
  firewallEnabled: boolean;
  agentRunning: boolean;
  serviceInstalled: boolean;
  localIpAddresses: string[];
  tlsActive: boolean;
  certFingerprint: string;
  lastIntegrityCheck: string;
  configurationIntact: boolean;
  recentSecurityEvents: SecurityEvent[];
}

export interface OverviewSummary {
  connectedChildrenCount: number;
  onlineDevicesCount: number;
  totalDevicesCount: number;
  todayTotalMinutes: number;
  blockedAttemptsToday: number;
  blockedDomainsCount: number;
  newlyInstalledCount: number;
  securityHealthy: boolean;
  recentApps: AppUsageEntry[];
  recentBlockedDomains: BlockedDomainEvent[];
  notifications: SecurityEvent[];
}

export interface ChildModeStatus {
  deviceId: string;
  deviceName: string;
  familyName: string;
  parentContact: string;
  parentEndpoint: string;
  connectedToLan: boolean;
  protectionActive: boolean;
  screenTimeEval: ScreenTimeEvaluation;
  screenTimePolicy: ScreenTimePolicy;
  appRules: AppRule[];
  websitePolicy: WebsitePolicy;
  lastSyncAt: string;
}

export interface BootstrapState {
  config: FamilyConfig;
  children: ChildProfile[];
  devices: Device[];
  overview: OverviewSummary;
  childStatus: ChildModeStatus;
  discoveredPeers: DiscoveryBeacon[];
  localLanIps: string[];
}

export interface PairingTicket {
  code: string;
  childId: string;
  childName: string;
  deviceNameHint: string;
  expiresAt: string;
  parentEndpoint: string;
  parentCertFingerprint: string;
}

async function callRPC<T>(method: string, args: Record<string, unknown> = {}): Promise<T> {
  const res = await fetch(`/rpc/${method}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(args),
  });
  const data = await res.json();
  if (!res.ok || data.error) {
    throw new Error(data.error || `RPC ${method} failed (${res.status})`);
  }
  return data.result as T;
}

export const api = {
  getBootstrapState: () => callRPC<BootstrapState>('GetBootstrapState'),

  setupParentMode: (familyName: string, password: string, parentContact: string, firstChildName: string) =>
    callRPC<{ config: FamilyConfig; recoveryCode: string; sessionToken: string }>('SetupParentMode', {
      familyName,
      password,
      parentContact,
      firstChildName,
    }),

  setupChildMode: (deviceName: string, parentPassword: string, parentContact: string) =>
    callRPC<FamilyConfig>('SetupChildMode', { deviceName, parentPassword, parentContact }),

  loginParent: (password: string) =>
    callRPC<{ token: string; role: string; expiresAt: string }>('LoginParent', { password }),

  logoutParent: (sessionToken: string) =>
    callRPC<{ ok: boolean }>('LogoutParent', { sessionToken }),

  recoverParentPassword: (recoveryCode: string, newPassword: string) =>
    callRPC<string>('RecoverParentPassword', { recoveryCode, newPassword }),

  changeParentPassword: (currentPassword: string, newPassword: string) =>
    callRPC<{ ok: boolean }>('ChangeParentPassword', { currentPassword, newPassword }),

  switchOperatingMode: (password: string, targetMode: OperatingMode) =>
    callRPC<FamilyConfig>('SwitchOperatingMode', { password, targetMode }),

  addChild: (name: string, avatarColor: string, ageGroup: string) =>
    callRPC<ChildProfile>('AddChild', { name, avatarColor, ageGroup }),

  removeChild: (childId: string) =>
    callRPC<{ ok: boolean }>('RemoveChild', { childId }),

  createPairingCode: (childId: string, deviceNameHint: string) =>
    callRPC<PairingTicket>('CreatePairingCode', { childId, deviceNameHint }),

  pairWithParent: (parentEndpoint: string, pairingCode: string, expectedCertFp = '') =>
    callRPC<{ deviceId: string; familyName: string }>('PairWithParent', {
      parentEndpoint,
      pairingCode,
      expectedCertFp,
    }),

  triggerChildSync: () => callRPC<unknown>('TriggerChildSync'),

  triggerDiscoveryBroadcast: () => callRPC<DiscoveryBeacon[]>('TriggerDiscoveryBroadcast'),

  renameDevice: (deviceId: string, newName: string, childId: string) =>
    callRPC<{ ok: boolean }>('RenameDevice', { deviceId, newName, childId }),

  revokeDevice: (deviceId: string) =>
    callRPC<{ ok: boolean }>('RevokeDevice', { deviceId }),

  deleteDevice: (deviceId: string) =>
    callRPC<{ ok: boolean }>('DeleteDevice', { deviceId }),

  getActivityBundle: (deviceId: string, dateStr = '') =>
    callRPC<{
      dailySummary: DailySummary;
      history7Days: DailySummary[];
      recentEvents: ActivityEvent[];
    }>('GetActivityBundle', { deviceId, dateStr }),

  getScreenTimeBundle: (deviceId: string) =>
    callRPC<{ policy: ScreenTimePolicy; evaluation: ScreenTimeEvaluation }>('GetScreenTimeBundle', { deviceId }),

  saveScreenTimePolicy: (policy: ScreenTimePolicy) =>
    callRPC<ScreenTimePolicy>('SaveScreenTimePolicy', { policy }),

  setDeviceLockAndPause: (deviceId: string, screenLocked: boolean, manualPause: boolean) =>
    callRPC<ScreenTimePolicy>('SetDeviceLockAndPause', { deviceId, screenLocked, manualPause }),

  grantBonusTime: (deviceId: string, bonusMinutes: number) =>
    callRPC<ScreenTimePolicy>('GrantBonusTime', { deviceId, bonusMinutes }),

  getApplicationsBundle: (deviceId: string) =>
    callRPC<{
      installedApps: InstalledApp[];
      rules: AppRule[];
      recentUsage: AppUsageEntry[];
    }>('GetApplicationsBundle', { deviceId }),

  refreshInstalledApplications: (deviceId: string) =>
    callRPC<InstalledApp[]>('RefreshInstalledApplications', { deviceId }),

  saveApplicationRule: (rule: Partial<AppRule>) =>
    callRPC<AppRule>('SaveApplicationRule', { rule }),

  deleteApplicationRule: (deviceId: string, executableName: string) =>
    callRPC<{ ok: boolean }>('DeleteApplicationRule', { deviceId, executableName }),

  acknowledgeNewlyInstalledApp: (deviceId: string, executableName: string) =>
    callRPC<{ ok: boolean }>('AcknowledgeNewlyInstalledApp', { deviceId, executableName }),

  getWebsitesBundle: (deviceId: string) =>
    callRPC<{
      policy: WebsitePolicy;
      verification: DNSVerificationStatus;
      blockedEvents: BlockedDomainEvent[];
    }>('GetWebsitesBundle', { deviceId }),

  saveWebsitePolicy: (policy: WebsitePolicy) =>
    callRPC<WebsitePolicy>('SaveWebsitePolicy', { policy }),

  enforceDnsAndHosts: (deviceId: string) =>
    callRPC<DNSVerificationStatus>('EnforceDNSAndHosts', { deviceId }),

  getProtectionStatus: (deviceId: string) =>
    callRPC<ProtectionStatus>('GetProtectionStatus', { deviceId }),

  updateGeneralSettings: (familyName: string, parentContact: string, retentionDays: number, listenPort: number) =>
    callRPC<FamilyConfig>('UpdateGeneralSettings', { familyName, parentContact, retentionDays, listenPort }),

  pruneHistoricalData: (retentionDays: number) =>
    callRPC<number>('PruneHistoricalData', { retentionDays }),

  exportEncryptedBackup: () => callRPC<string>('ExportEncryptedBackup'),

  importEncryptedBackup: (encryptedPayload: string) =>
    callRPC<{ ok: boolean }>('ImportEncryptedBackup', { encryptedPayload }),

  authorizeUninstall: (parentPassword: string) =>
    callRPC<boolean>('AuthorizeUninstall', { parentPassword }),
};
