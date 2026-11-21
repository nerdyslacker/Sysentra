<script lang="ts">
  import { onMount } from 'svelte';
  import { Browser } from '@wailsio/runtime';

  import type { AccountHistoryView, AdditionalSourceView, DriverDeviceView, FirewallChangeInput, FirewallRuleView, FirewallView, FirmwareDeviceView, FlatpakRemoteView, GroupView, KernelStatusView, KernelView, MirrorView, NFSMountView, PackageDetailsView, PackageTransactionPlanView, PackageView, RepositoryKeyView, RepositoryView, SambaServerView, SambaShareView, ServiceDetailsView, ServiceView, TransactionHistoryView, UserView, XBPSConfigRuleView } from '../bindings/sysentra/internal/app/models';
  import { Discover, FindOwningPackage, GetPackageDetails, ListAdditionalSources, ListInstalled, ListUpdates, SetAdditionalSourceEnabled } from '../bindings/sysentra/internal/app/packageservice';
  import { ListSystemFonts } from '../bindings/sysentra/internal/app/fontservice';
  import { AuthorizeSession, CommitPackage, ListTransactionHistory, PreviewDatabaseSync, PreviewDowngrade, PreviewFlatpakCleanup, PreviewMaintenance, PreviewPackage, PreviewPackages, PreviewSystemUpgrade } from '../bindings/sysentra/internal/app/packagetransactionservice';
  import { ChangeFlatpakRemote, ListFlatpakRemotes } from '../bindings/sysentra/internal/app/flatpakremoteservice';
  import { InspectRepositoryKey } from '../bindings/sysentra/internal/app/repositorytrustservice';
  import { AddService, AssignAccountGroups, ChangeAccount, ChangeFirewall, ChangeFirmware, ChangeMirror, ChangeNFSMount, ChangeRepository, ChangeSambaGlobal, ChangeSambaShare, ChangeService, ChangeTurnstileReady, ChangeTurnstileService, ChangeUserService, ChangeXBPSRule, CreateUser, EditUser, GetFirewall, GetKernelStatus, GetMirrorRecovery, GetSambaServer, GetServiceDetails, GetSystemInfo, InitializeUserServices, ListAccountHistory, ListAccounts, ListDrivers, ListFirmware, ListKernels, ListMirrors, ListNFSClient, ListRepositories, ListServices, ListTurnstileServices, ListUserServices, ListXBPSConfigRules, ReconfigureKernel, SetDefaultKernel, SetMirrorRecovery, UpdateTurnstileEnvironment } from '../bindings/sysentra/internal/app/systemservice';
  import logoUrl from '../../assets/sysentra.png';
  import VirtualPackageTable from './components/VirtualPackageTable.svelte';
  import PackageDetailsSidebar from './components/PackageDetailsSidebar.svelte';
  import NavigationIcon from './components/NavigationIcon.svelte';
  import ServiceDetailsSidebar from './components/ServiceDetailsSidebar.svelte';
  import { updatePackageAfterTransaction } from './lib/package-list.js';

  const navigation = ['Discover', 'Installed', 'Updates', 'Repositories', 'XBPS Config', 'Mirrors', 'Services', 'Users & Groups', 'Firewall', 'Samba Server', 'NFS Client', 'Kernels', 'Firmware', 'Drivers'];
  const driverCategories = ['All', 'Video', 'Wi-Fi', 'Ethernet', 'Bluetooth', 'Audio', 'Webcam', 'Touchscreen', 'Printers', 'Scanners', 'Other'];
  const themes = [['default', 'Default'], ['catppuccin', 'Catppuccin'], ['everforest', 'Everforest'], ['github', 'GitHub'], ['gruvbox', 'Gruvbox'], ['one-dark', 'One Dark'], ['dracula', 'Dracula'], ['sepia', 'Sepia'], ['nord', 'Nord'], ['material', 'Material']];
  const sambaPermissions = [['0744', '744 – rwx r-- r--'], ['0755', '755 – rwx r-x r-x'], ['0770', '770 – rwx rwx ---'], ['0775', '775 – rwx rwx r-x'], ['0777', '777 – rwx rwx rwx']];

  let activeView = 'Discover';
  let packages: PackageView[] = [];
  let catalogWarnings: string[] = [];
  let packageCache: Record<string, { packages: PackageView[]; warnings: string[] }> = {};
  let packagesLoading = true;
  let packageError = '';
  let activeRequest: ReturnType<typeof ListInstalled> | null = null;
  let services: ServiceView[] = [];
  let kernels: KernelView[] = [];
  let kernelStatus: KernelStatusView | null = null;
  let kernelTab: 'packages' | 'images' | 'dkms' = 'packages';
  let repositories: RepositoryView[] = [];
  let flatpakRemotes: FlatpakRemoteView[] = [];
  let xbpsConfigRules: XBPSConfigRuleView[] = [];
  let mirrors: MirrorView[] = [];
  let mirrorWarnings: string[] = [];
  let mirrorTier = 0;
  let mirrorRegion = 'All';
  let mirrorRecoveryEnabled = false;
  let mirrorRecoverySaving = false;
  let mirrorRecoveryError = '';
  let firmwareDevices: FirmwareDeviceView[] = [];
  let firmwareAvailable = false;
  let firmwareWarning = '';
  let firmwareDialog: HTMLDialogElement;
  let firmwareOperation: 'refresh' | 'update' = 'refresh';
  let firmwareDevice: FirmwareDeviceView | null = null;
  let firmwareActionLoading = false;
  let firmwareActionError = '';
  let firmwareActionLogs: string[] = [];
  let firmwareActionSucceeded = false;
  let firewall: FirewallView | null = null;
  let firewallCache: Record<string, FirewallView> = {};
  let firewallSelectedBackend = '';
  let firewallContentLoading = false;
  let firewallContentError = '';
  let firewallRequestID = 0;
  let firewallDialog: HTMLDialogElement;
  let firewallDialogMode: 'add' | 'edit' | 'policy' | 'zone' = 'add';
  let firewallOldRule: FirewallRuleView | null = null;
  let firewallRuleKind = 'port';
  let firewallTarget = '';
  let firewallProtocol = 'tcp';
  let firewallRuleDirection = 'in';
  let firewallSource = 'any';
  let firewallAction = 'allow';
  let firewallPolicy = 'deny';
  let firewallDirection = 'incoming';
  let firewallZone = '';
  let firewallSaving = false;
  let firewallError = '';
  let driverDevices: DriverDeviceView[] = [];
  let driverWarnings: string[] = [];
  let driverCategory = 'All';
  let sambaServer: SambaServerView | null = null;
  let nfsMounts: NFSMountView[] = [];
  let users: UserView[] = [];
  let groups: GroupView[] = [];
  let accountTab: 'users' | 'groups' = 'users';
  let accountDialog: HTMLDialogElement;
  let accountOperation = 'user-add';
  let accountUser = '';
  let accountGroup = '';
  let accountGroups: string[] = [];
  let accountCreateHome = true;
  let accountHome = '';
  let accountShell = '';
  let accountMoveHome = false;
  let accountHistoryDialog: HTMLDialogElement;
  let accountHistory: AccountHistoryView[] = [];
  let accountSaving = false;
  let accountError = '';
  let systemLoading = false;
  let systemError = '';
  let systemRequestID = 0;
  let selectedService: ServiceView | null = null;
  let serviceDetails: ServiceDetailsView | null = null;
  let serviceDetailsLoading = false;
  let serviceDetailsError = '';
  let serviceScope: 'system' | 'user' | 'turnstile' = 'system';
  const serviceTimeout = 7;
  let serviceFilter: 'all' | 'active' | 'disabled' = 'all';
  let turnstileReady = '';
  let turnstileEnvironment = '';
  let serviceDialog: HTMLDialogElement;
  let kernelReconfigureDialog: HTMLDialogElement;
  let kernelReconfigureTitle = '';
  let kernelReconfigureLogs: string[] = [];
  let kernelReconfigureLoading = false;
  let kernelReconfigureError = '';
  let kernelReconfigureSucceeded = false;
  let serviceName = '';
  let serviceCommand = '';
  let serviceUser = '';
  let serviceGroup = '';
  let serviceSaving = false;
  let serviceDialogError = '';
  let updateCount = 0;
  let packageResultCount = 0;
  let systemName = 'Linux';
  let searchInput = '';
  let globalQuery = '';
  let searchTimer: ReturnType<typeof setTimeout>;
  let confirmationDialog: HTMLDialogElement;
  let confirmation = { title: '', description: '', destructive: false };
  let transactionPlan: PackageTransactionPlanView | null = null;
  let transactionLoading = false;
  let transactionError = '';
  let transactionOutcome: 'success' | 'failure' | '' = '';
  let transactionExecutionLogs: string[] = [];
  let transactionLogsVisible = false;
  let transactionSequence = 0;
  let confirmationAction: (() => Promise<void>) | null = null;
  let historyDialog: HTMLDialogElement;
  let transactionHistory: TransactionHistoryView[] = [];
  let historyError = '';
  let ownerDialog: HTMLDialogElement;
  let ownerQuery = '';
  let ownerMatches: string[] = [];
  let ownerError = '';
  let ownerLoading = false;
  let ownerSearched = false;
  let ruleDialog: HTMLDialogElement;
  let ruleOperation = 'add';
  let ruleKind = 'ignorepkg';
  let ruleValue = '';
  let ruleOldValue = '';
  let ruleSaving = false;
  let ruleError = '';
  let configDialog: HTMLDialogElement;
  let configKind: 'repository' | 'flatpak' | 'samba' | 'nfs' = 'repository';
  let configOperation = 'add';
  let configName = '';
  let configValue = '';
  let configOldValue = '';
  let configPath = '';
  let configOldPath = '';
  let configType = 'nfs4';
  let configOptions = 'rw,_netdev';
  let configReadOnly = true;
  let configGuestOK = false;
  let configComment = '';
  let configBrowseable = true;
  let configFollowSymlinks = true;
  let configWideLinks = false;
  let configValidUsers = '';
  let configWriteList = '';
  let configReadList = '';
  let configValidGroups: string[] = [];
  let configWriteGroups: string[] = [];
  let configReadGroups: string[] = [];
  let configForceUser = '';
  let configForceGroup = '';
  let configCreateMask = '0744';
  let configDirectoryMask = '0755';
  let configSaving = false;
  let configError = '';
  let sambaGlobalDialog: HTMLDialogElement;
  let sambaGlobalProtocol = 'SMB2';
  let sambaGlobalMask = '0755';
  let sambaGlobalPrinting = false;
  let sambaGlobalMapGuest = false;
  let sambaGlobalFirewallAllowed = false;
  let sambaGlobalSaving = false;
  let sambaGlobalError = '';
  let configTrustConfirmed = false;
  let configRepositoryKey: RepositoryKeyView | null = null;
  let configKeyURL = '';
  let configKeyLoading = false;
  let selectedPackage: PackageView | null = null;
  let selectedPackages: PackageView[] = [];
  let packageDetails: PackageDetailsView | null = null;
  let packageDetailsLoading = false;
  let packageDetailsError = '';
  let activeDetailsRequest: ReturnType<typeof GetPackageDetails> | null = null;
  let colorMode: 'auto' | 'light' | 'dark' = 'auto';
  let darkMode = false;
  let systemColorMode: MediaQueryList;
  let settingsDialog: HTMLDialogElement;
  let theme = 'default';
  let font = 'system';
  let fontSize = 16;
  let systemFonts: string[] = [];
  let additionalSources: AdditionalSourceView[] = [];
  let sourceSaving = '';
  let settingsError = '';

  $: normalizedQuery = globalQuery.trim().toLowerCase();
  $: flatpakSourceEnabled = additionalSources.find((source) => source.id === 'flatpak')?.enabled !== false;
  $: transactionLogs = [...(transactionPlan?.diagnostics ?? []), ...transactionExecutionLogs];
  $: filteredServices = services.filter((item) => (serviceFilter === 'all' || (serviceFilter === 'active' && item.status === 'Running') || (serviceFilter === 'disabled' && !item.enabled)) && [item.name, item.status, item.command, String(item.pid)].some((value) => value.toLowerCase().includes(normalizedQuery)));
  $: filteredKernels = kernels.filter((item) => [item.package, item.version, item.series, item.type, item.size, item.installDate, item.running ? 'running current' : item.installed ? 'installed' : 'available'].some((value) => value.toLowerCase().includes(normalizedQuery)));
  $: purgeableKernelImages = kernelStatus?.images.filter((image) => image.canPurge) ?? [];
  $: filteredRepositories = repositories.filter((item) => [item.url, item.origin].some((value) => value.toLowerCase().includes(normalizedQuery)));
  $: filteredFlatpakRemotes = flatpakRemotes.filter((item) => [item.name, item.title, item.url].some((value) => value.toLowerCase().includes(normalizedQuery)));
  $: filteredXBPSConfigRules = xbpsConfigRules.filter((item) => [item.kind, item.value, item.origin].some((value) => value.toLowerCase().includes(normalizedQuery)));
  $: mirrorRegions = ['All', ...new Set(mirrors.map((item) => item.region))];
  $: filteredMirrors = mirrors.filter((item) => (!mirrorTier || item.tier === mirrorTier) && (mirrorRegion === 'All' || item.region === mirrorRegion) && [item.url, item.region, item.location, item.status].some((value) => value.toLowerCase().includes(normalizedQuery)));
  $: filteredFirmwareDevices = firmwareDevices.filter((item) => [item.name, item.vendor, item.summary, item.version, item.newVersion, item.id].some((value) => value.toLowerCase().includes(normalizedQuery)));
  $: filteredFirewallRules = (firewall?.rules ?? []).filter((item) => [item.target, item.action, item.source].some((value) => value.toLowerCase().includes(normalizedQuery)));
  $: filteredDriverDevices = driverDevices.filter((item) => (driverCategory === 'All' || item.category === driverCategory) && [item.category, item.bus, item.id, item.name, item.driver, item.status, item.evidence].some((value) => value.toLowerCase().includes(normalizedQuery)));
  $: filteredSambaShares = (sambaServer?.shares ?? []).filter((item) => [item.name, item.path, item.comment, item.validUsers, item.writeList, item.readList, item.readOnly ? 'read only' : 'writable', item.guestOK ? 'guest' : 'authenticated'].some((value) => value.toLowerCase().includes(normalizedQuery)));
  $: filteredNFSMounts = nfsMounts.filter((item) => [item.source, item.target, item.type, item.options, item.mounted ? 'mounted' : 'unmounted'].some((value) => value.toLowerCase().includes(normalizedQuery)));
  $: filteredUsers = users.filter((item) => [item.name, item.home, item.shell, String(item.uid), ...item.groups].some((value) => value.toLowerCase().includes(normalizedQuery)));
  $: filteredGroups = groups.filter((item) => [item.name, String(item.gid), ...item.members].some((value) => value.toLowerCase().includes(normalizedQuery)));

  onMount(() => {
    void AuthorizeSession().catch(() => {});
    let savedMode: string | null = null;
    try {
      savedMode = localStorage.getItem('sysentra-color-mode');
      theme = themes.some(([id]) => id === localStorage.getItem('sysentra-theme')) ? localStorage.getItem('sysentra-theme')! : 'default';
      font = localStorage.getItem('sysentra-font') || 'system';
      const savedSize = Number(localStorage.getItem('sysentra-font-size'));
      fontSize = savedSize >= 10 && savedSize <= 24 ? savedSize : 16;
    } catch { /* storage can be disabled */ }
    systemColorMode = window.matchMedia('(prefers-color-scheme: dark)');
    const mode = savedMode === 'light' || savedMode === 'dark' ? savedMode : 'auto';
    setColorMode(mode, false);
    applyAppearance(false);
    const followSystemMode = () => { if (colorMode === 'auto') applyColorMode(); };
    const usePointerFocus = () => document.documentElement.dataset.inputMethod = 'pointer';
    const useKeyboardFocus = (event: KeyboardEvent) => { if (event.key === 'Tab') document.documentElement.dataset.inputMethod = 'keyboard'; };
    systemColorMode.addEventListener('change', followSystemMode);
    window.addEventListener('pointerdown', usePointerFocus, true);
    window.addEventListener('keydown', useKeyboardFocus, true);
    restoreAdditionalSources();
    loadSystemFonts();
    GetSystemInfo().then((info) => systemName = info.name).catch(() => {});
    return () => {
      void activeRequest?.cancel();
      void activeDetailsRequest?.cancel();
      clearTimeout(searchTimer);
      systemColorMode.removeEventListener('change', followSystemMode);
      window.removeEventListener('pointerdown', usePointerFocus, true);
      window.removeEventListener('keydown', useKeyboardFocus, true);
    };
  });

  function selectView(view: string) {
    clearTimeout(searchTimer);
    searchInput = '';
    globalQuery = '';
    closePackageDetails();
    activeView = view;
    if (view === 'Discover' || view === 'Installed' || view === 'Updates') {
      systemRequestID += 1;
      if (!showCachedPackages(view)) loadPackages(view);
    } else if (view === 'Services' || view === 'Users & Groups' || view === 'Firewall' || view === 'Samba Server' || view === 'NFS Client' || view === 'Kernels' || view === 'Repositories' || view === 'XBPS Config' || view === 'Mirrors' || view === 'Firmware' || view === 'Drivers') {
      loadSystemView(view);
    } else {
      systemRequestID += 1;
      void activeRequest?.cancel();
      activeRequest = null;
      packagesLoading = false;
      packageError = '';
      packages = [];
      catalogWarnings = [];
      systemError = '';
    }
  }

  async function loadSystemView(view = activeView) {
    if (view === 'Firewall' && firewallSelectedBackend && firewallCache[firewallSelectedBackend]) {
      systemRequestID += 1;
      systemLoading = false;
      systemError = '';
      firewall = firewallCache[firewallSelectedBackend];
      return;
    }
    const requestID = ++systemRequestID;
    systemLoading = true;
    systemError = '';
    selectedService = null;
    try {
      if (view === 'Services') {
        const result = await (serviceScope === 'system' ? ListServices() : serviceScope === 'user' ? ListUserServices() : ListTurnstileServices());
        if (requestID === systemRequestID) services = result;
      }
      if (view === 'Kernels') {
        const [result, status] = await Promise.all([ListKernels(), GetKernelStatus()]);
        if (requestID === systemRequestID) {
          kernels = result;
          kernelStatus = status;
        }
      }
      if (view === 'Repositories') {
        const [result, remotes] = await Promise.all([ListRepositories(), flatpakSourceEnabled ? ListFlatpakRemotes().catch(() => []) : Promise.resolve([])]);
        if (requestID === systemRequestID) {
          repositories = result;
          flatpakRemotes = remotes;
        }
      }
      if (view === 'XBPS Config') {
        const result = await ListXBPSConfigRules();
        if (requestID === systemRequestID) xbpsConfigRules = result;
      }
      if (view === 'Mirrors') {
        const [result, recovery] = await Promise.all([ListMirrors(), GetMirrorRecovery()]);
        if (requestID === systemRequestID) {
          mirrors = result.mirrors ?? [];
          mirrorWarnings = result.warnings ?? [];
          mirrorRecoveryEnabled = recovery.enabled;
          mirrorRecoveryError = '';
        }
      }
      if (view === 'Firmware') {
        const result = await ListFirmware();
        if (requestID === systemRequestID) {
          firmwareAvailable = result.available;
          firmwareDevices = result.devices;
          firmwareWarning = result.warning;
        }
      }
      if (view === 'Firewall') {
        const result = await GetFirewall(firewallSelectedBackend);
        if (requestID === systemRequestID) storeFirewall(result);
      }
      if (view === 'Drivers') {
        const result = await ListDrivers();
        if (requestID === systemRequestID) {
          driverDevices = result.devices;
          driverWarnings = result.warnings;
        }
      }
      if (view === 'Samba Server') {
        const result = await GetSambaServer();
        if (requestID === systemRequestID) sambaServer = result;
      }
      if (view === 'NFS Client') {
        const result = await ListNFSClient();
        if (requestID === systemRequestID) nfsMounts = result;
      }
      if (view === 'Users & Groups') {
        const result = await ListAccounts();
        if (requestID === systemRequestID) { users = result.users; groups = result.groups; }
      }
    } catch (error) {
      if (requestID === systemRequestID) systemError = error instanceof Error ? error.message : String(error);
    } finally {
      if (requestID === systemRequestID) systemLoading = false;
    }
  }

  function toggleSelection(selected: Set<string>, value: string) {
    const next = new Set(selected);
    if (next.has(value)) next.delete(value); else next.add(value);
    return next;
  }

  function loadPackages(view = activeView, silent = false) {
    activeRequest?.cancel();
    packageError = '';
    if (!silent) packagesLoading = true;
    const request = view === 'Discover' ? Discover() : view === 'Updates' ? ListUpdates() : ListInstalled();
    activeRequest = request;
    request
      .then((result) => {
        if (activeRequest === request) {
          packageCache[view] = { packages: result.packages, warnings: result.warnings };
          packages = result.packages;
          packageResultCount = result.packages.length;
          catalogWarnings = result.warnings;
          if (view === 'Updates') updateCount = result.packages.length;
          if (selectedPackage) {
            selectedPackage = result.packages.find((pkg) => pkg.backendId === selectedPackage?.backendId && pkg.id === selectedPackage?.id) ?? selectedPackage;
          }
        }
      })
      .catch((error) => {
        if (activeRequest === request) packageError = error instanceof Error ? error.message : String(error);
      })
      .finally(() => {
        if (activeRequest === request && !silent) packagesLoading = false;
      });
  }

  function showCachedPackages(view: string) {
    const cached = packageCache[view];
    if (!cached) return false;
    void activeRequest?.cancel();
    activeRequest = null;
    packages = cached.packages;
    catalogWarnings = cached.warnings;
    packageResultCount = cached.packages.length;
    packageError = '';
    packagesLoading = false;
    return true;
  }

  function invalidatePackageCache() {
    packageCache = {};
  }

  function refreshUpdateCount() {
    ListUpdates().then((result) => {
      updateCount = result.packages.length;
      packageCache.Updates = { packages: result.packages, warnings: result.warnings };
    }).catch(() => {});
  }

  function reloadPackages() {
    closePackageDetails();
    loadPackages();
    if (activeView !== 'Updates') refreshUpdateCount();
  }

  function closePackageDetails() {
    void activeDetailsRequest?.cancel();
    activeDetailsRequest = null;
    selectedPackage = null;
    packageDetails = null;
    packageDetailsError = '';
    packageDetailsLoading = false;
  }

  function openPackageDetails(pkg: PackageView) {
    void activeDetailsRequest?.cancel();
    selectedPackage = pkg;
    packageDetails = null;
    packageDetailsError = '';
    packageDetailsLoading = true;
    const request = GetPackageDetails(pkg.backendId, pkg.id);
    activeDetailsRequest = request;
    request.then((details) => {
      if (activeDetailsRequest === request) packageDetails = details;
    }).catch((error) => {
      if (activeDetailsRequest === request) packageDetailsError = error instanceof Error ? error.message : String(error);
    }).finally(() => {
      if (activeDetailsRequest === request) packageDetailsLoading = false;
    });
  }

  function openConfirmation(title: string, description: string, destructive = false, operation = '', action: (() => Promise<void>) | null = null, targetVersion = '') {
    confirmation = { title, description, destructive };
    transactionPlan = null;
    transactionError = '';
    transactionOutcome = '';
    transactionExecutionLogs = [];
    transactionLogsVisible = false;
    transactionLoading = false;
    confirmationAction = action;
    confirmationDialog.showModal();
    if (operation && selectedPackage) void previewPackageTransaction(selectedPackage, operation, targetVersion);
  }

  async function previewPackageTransaction(pkg: PackageView, operation: string, targetVersion = '') {
    const sequence = ++transactionSequence;
    transactionLoading = true;
    try {
      const plan = operation === 'downgrade' ? await PreviewDowngrade(pkg.id, targetVersion) : await PreviewPackage(pkg.backendId, pkg.id, operation, pkg.scope);
      if (sequence === transactionSequence) transactionPlan = plan;
    } catch (error) {
      if (sequence === transactionSequence) transactionError = error instanceof Error ? error.message : String(error);
    } finally {
      if (sequence === transactionSequence) transactionLoading = false;
    }
  }

  async function openSelectedPackageTransaction(action: 'install' | 'remove' | 'update' | 'force-remove') {
    if (selectedPackages.length === 0) return;
    if (action === 'force-remove') {
      const packageIDs = selectedPackages.filter((pkg) => pkg.backendId === 'xbps' && pkg.installed).map((pkg) => pkg.id);
      if (packageIDs.length) await openMaintenance('Force remove selected', 'Emergency removal; dependency damage must be reviewed', action, true, packageIDs);
      return;
    }
    const verb = action === 'install' ? 'Install' : action === 'remove' ? 'Remove' : 'Update';
    const backend = selectedPackages[0].backendId;
    confirmation = { title: `${verb} selected packages`, description: `${verb} ${selectedPackages.length} selected ${backend === 'flatpak' ? 'Flatpak' : 'XBPS'} packages.`, destructive: action === 'remove' };
    transactionPlan = null;
    transactionError = '';
    transactionOutcome = '';
    transactionExecutionLogs = [];
    transactionLogsVisible = false;
    confirmationAction = null;
    transactionLoading = true;
    confirmationDialog.showModal();
    const sequence = ++transactionSequence;
    try {
      const plan = await PreviewPackages(backend, selectedPackages.map((pkg) => pkg.id), action, selectedPackages[0].scope);
      if (sequence === transactionSequence) transactionPlan = plan;
    } catch (error) {
      if (sequence === transactionSequence) transactionError = error instanceof Error ? error.message : String(error);
    } finally {
      if (sequence === transactionSequence) transactionLoading = false;
    }
  }

  async function openFlatpakCleanup() {
    confirmation = { title: 'Remove unused Flatpak runtimes', description: 'Remove user-scoped Flatpak runtimes and extensions no longer required by installed applications.', destructive: true };
    transactionPlan = null;
    transactionError = '';
    transactionOutcome = '';
    transactionExecutionLogs = [];
    transactionLogsVisible = false;
    confirmationAction = null;
    transactionLoading = true;
    confirmationDialog.showModal();
    try {
      transactionPlan = await PreviewFlatpakCleanup();
    } catch (error) {
      transactionError = error instanceof Error ? error.message : String(error);
    } finally {
      transactionLoading = false;
    }
  }

  async function openMaintenance(title: string, description: string, operation: string, destructive: boolean, packageIDs: string[] = []) {
    confirmation = { title, description, destructive };
    transactionPlan = null;
    transactionError = '';
    transactionOutcome = '';
    transactionExecutionLogs = [];
    transactionLogsVisible = false;
    confirmationAction = null;
    transactionLoading = true;
    confirmationDialog.showModal();
    const sequence = ++transactionSequence;
    try {
      const plan = await PreviewMaintenance(operation, packageIDs);
      if (sequence === transactionSequence) transactionPlan = plan;
    } catch (error) {
      if (sequence === transactionSequence) transactionError = error instanceof Error ? error.message : String(error);
    } finally {
      if (sequence === transactionSequence) transactionLoading = false;
    }
  }

  function openMaintenanceTool(name: string, description: string) {
    if (name === 'Find owning package') return openOwnerDialog();
    if (name === 'Clean unused Flatpaks') return void openFlatpakCleanup();
    if (name === 'Remove orphans') return void openMaintenance(name, description, 'remove-orphans', true);
    if (name === 'Clean package cache') return void openMaintenance(name, description, 'clean-cache', true);
    if (name === 'Verify package database') return void openMaintenance(name, description, 'verify-database', false);
    if (name === 'Reconfigure packages') return void openMaintenance(name, description, 'reconfigure-all', false);
  }

  async function openNamedPackageTransaction(title: string, description: string, packageNames: string[], action: 'install' | 'remove' | 'purge' | 'reconfigure') {
    if (packageNames.length === 0) return;
    confirmation = { title, description, destructive: action === 'remove' || action === 'purge' };
    transactionPlan = null;
    transactionError = '';
    transactionOutcome = '';
    transactionExecutionLogs = [];
    transactionLogsVisible = false;
    confirmationAction = null;
    transactionLoading = true;
    confirmationDialog.showModal();
    const sequence = ++transactionSequence;
    try {
      const plan = await PreviewPackages('xbps', packageNames, action, '');
      if (sequence === transactionSequence) transactionPlan = plan;
    } catch (error) {
      if (sequence === transactionSequence) transactionError = error instanceof Error ? error.message : String(error);
    } finally {
      if (sequence === transactionSequence) transactionLoading = false;
    }
  }

  function openKernelPurge() {
    if (purgeableKernelImages.length) {
      void openMaintenance('Purge old kernels', 'Remove obsolete kernel boot images reported by vkpurge. The running kernel is never included.', 'purge-kernels', true, purgeableKernelImages.map((image) => image.version));
      return;
    }
    const removable = kernels.filter((kernel) => kernel.canRemove).map((kernel) => kernel.package);
    void openNamedPackageTransaction('Purge old kernels', 'Remove old kernel packages while preserving the running kernel and at least one verified fallback.', removable, 'purge');
  }

  async function reconfigureKernel(item: KernelView) {
    if (kernelReconfigureLoading) return;
    kernelReconfigureTitle = `Reconfigure ${item.package}`;
    kernelReconfigureLogs = [`Running post-install hooks for ${item.package}…`];
    kernelReconfigureError = '';
    kernelReconfigureSucceeded = false;
    kernelReconfigureLoading = true;
    kernelReconfigureDialog.showModal();
    try {
      const result = await ReconfigureKernel(item.package);
      kernelReconfigureLogs = result.diagnostics.length ? result.diagnostics : ['Command completed without output.'];
      kernelReconfigureSucceeded = true;
    } catch (error) {
      kernelReconfigureError = error instanceof Error ? error.message : String(error);
    } finally {
      kernelReconfigureLoading = false;
    }
  }

  async function openServiceDetails(service: ServiceView) {
    const scope = serviceScope;
    selectedService = service;
    serviceDetails = null;
    serviceDetailsError = '';
    serviceDetailsLoading = true;
    try {
      const details = await GetServiceDetails(scope, service.name);
      if (selectedService?.name === service.name && serviceScope === scope) serviceDetails = details;
    } catch (error) {
      if (selectedService?.name === service.name && serviceScope === scope) serviceDetailsError = error instanceof Error ? error.message : String(error);
    } finally {
      if (selectedService?.name === service.name && serviceScope === scope) serviceDetailsLoading = false;
    }
  }

  function confirmDefaultKernel(item: KernelView) {
    openConfirmation('Set default kernel', `Boot ${item.package} (${item.version}) by default after the next restart.`, false, '', async () => {
      kernels = await SetDefaultKernel(item.package);
      kernelStatus = await GetKernelStatus();
    });
  }

  async function openSystemUpgrade() {
    confirmation = { title: 'Full system upgrade', description: 'Upgrade all packages with available updates.', destructive: false };
    transactionPlan = null;
    transactionError = '';
    transactionOutcome = '';
    transactionExecutionLogs = [];
    transactionLogsVisible = false;
    confirmationAction = null;
    transactionLoading = true;
    confirmationDialog.showModal();
    const sequence = ++transactionSequence;
    try {
      const plan = await PreviewSystemUpgrade();
      if (sequence === transactionSequence) transactionPlan = plan;
    } catch (error) {
      if (sequence === transactionSequence) transactionError = error instanceof Error ? error.message : String(error);
    } finally {
      if (sequence === transactionSequence) transactionLoading = false;
    }
  }

  async function openDatabaseSync() {
    confirmation = { title: 'Sync package database', description: 'Refresh XBPS repository indexes.', destructive: false };
    transactionPlan = null;
    transactionError = '';
    transactionOutcome = '';
    transactionExecutionLogs = [];
    transactionLogsVisible = false;
    confirmationAction = null;
    transactionLoading = true;
    confirmationDialog.showModal();
    const sequence = ++transactionSequence;
    try {
      const plan = await PreviewDatabaseSync();
      if (sequence === transactionSequence) transactionPlan = plan;
    } catch (error) {
      if (sequence === transactionSequence) transactionError = error instanceof Error ? error.message : String(error);
    } finally {
      if (sequence === transactionSequence) transactionLoading = false;
    }
  }

  async function commitPackageTransaction() {
    if (!transactionPlan || transactionLoading) return;
    transactionLoading = true;
    transactionError = '';
    transactionOutcome = '';
    try {
      const operation = transactionPlan.operation;
      const changedPackages = transactionPlan.changes.map((change) => change.name);
      const result = await CommitPackage(transactionPlan.requestId);
      transactionExecutionLogs = result.diagnostics ?? [];
      transactionOutcome = 'success';
      if (selectedPackage) {
        const updated = updatePackageAfterTransaction(selectedPackage, operation, changedPackages) as PackageView;
        if (updated !== selectedPackage) openPackageDetails(updated);
      }
      invalidatePackageCache();
      if (activeView === 'Discover' || activeView === 'Installed' || activeView === 'Updates') void loadPackages(activeView, true);
      if (activeView !== 'Updates') refreshUpdateCount();
      if (activeView !== 'Discover' && activeView !== 'Installed' && activeView !== 'Updates') await loadSystemView(activeView);
    } catch (error) {
      transactionError = error instanceof Error ? error.message : String(error);
      transactionExecutionLogs = [transactionError];
      transactionOutcome = 'failure';
    } finally {
      transactionLoading = false;
    }
  }

  async function executeConfirmation() {
    if (transactionPlan) {
      await commitPackageTransaction();
      return;
    }
    if (!confirmationAction || transactionLoading) return;
    transactionLoading = true;
    transactionError = '';
    try {
      await confirmationAction();
      confirmationDialog.close();
    } catch (error) {
      transactionError = error instanceof Error ? error.message : String(error);
    } finally {
      transactionLoading = false;
    }
  }

  function confirmServiceOperation(service: ServiceView | null, operation: string) {
    if (!service) return;
    const destructive = operation === 'stop' || operation === 'disable' || operation === 'delete';
    openConfirmation(
      `${operation[0].toUpperCase()}${operation.slice(1)} ${service.name}`,
      `${operation[0].toUpperCase()}${operation.slice(1)} the runit service ${service.name}.`,
      destructive,
      '',
      async () => {
        services = await (serviceScope === 'system' ? ChangeService(service.name, operation, serviceTimeout) : serviceScope === 'user' ? ChangeUserService(service.name, operation, serviceTimeout) : ChangeTurnstileService(service.name, operation, serviceTimeout));
        const updated = services.find((item) => item.name === service.name);
        if (updated) await openServiceDetails(updated);
        else if (operation === 'delete') {
          selectedService = null;
          serviceDetails = null;
        }
      }
    );
  }

  function selectServiceScope(scope: 'system' | 'user' | 'turnstile') {
    if (scope === serviceScope) return;
    serviceScope = scope;
    void loadSystemView('Services');
  }

  function initializeUserServices() {
    openConfirmation('Initialize user services', 'Create and enable a system runsvdir service for the current user.', false, '', async () => {
      await InitializeUserServices();
      services = await ListUserServices();
    });
  }

  function openServiceDialog() {
    serviceName = '';
    serviceCommand = '';
    serviceUser = '';
    serviceGroup = '';
    serviceDialogError = '';
    serviceDialog.showModal();
  }

  async function submitService() {
    if (serviceSaving || serviceScope === 'turnstile') return;
    serviceSaving = true;
    serviceDialogError = '';
    try {
      services = await AddService(serviceScope, serviceName.trim(), serviceCommand.trim(), serviceUser.trim(), serviceGroup.trim());
      serviceDialog.close();
    } catch (error) {
      serviceDialogError = error instanceof Error ? error.message : String(error);
    } finally {
      serviceSaving = false;
    }
  }

  function changeTurnstileReady(operation: 'add' | 'remove') {
    const names = turnstileReady.split(/[\s,]+/).filter(Boolean);
    if (!names.length) return;
    openConfirmation(`${operation === 'add' ? 'Add' : 'Remove'} ready services`, `${operation === 'add' ? 'Add to' : 'Remove from'} turnstile core services: ${names.join(', ')}.`, operation === 'remove', '', async () => {
      await ChangeTurnstileReady(operation, names);
      turnstileReady = '';
    });
  }

  function updateTurnstileEnvironment() {
    const values = turnstileEnvironment.split(/[\n,]+/).map((value) => value.trim()).filter(Boolean);
    if (!values.length) return;
    openConfirmation('Update turnstile environment', `Update ${values.length} shared environment ${values.length === 1 ? 'variable' : 'variables'}.`, false, '', async () => {
      await UpdateTurnstileEnvironment(values);
      turnstileEnvironment = '';
    });
  }

  function ruleKindID(rule: XBPSConfigRuleView) {
    return rule.kind === 'Ignored package' ? 'ignorepkg' : 'virtualpkg';
  }

  function openRuleDialog(operation = 'add', rule: XBPSConfigRuleView | null = null) {
    ruleOperation = operation;
    ruleKind = rule ? ruleKindID(rule) : 'ignorepkg';
    ruleValue = rule?.value ?? '';
    ruleOldValue = rule?.value ?? '';
    ruleError = '';
    ruleDialog.showModal();
  }

  async function saveRule() {
    if (!ruleValue.trim() || ruleSaving) return;
    ruleSaving = true;
    ruleError = '';
    try {
      xbpsConfigRules = await ChangeXBPSRule(ruleOperation, ruleKind, ruleValue.trim(), ruleOperation === 'edit' ? ruleOldValue : '');
      ruleDialog.close();
    } catch (error) {
      ruleError = error instanceof Error ? error.message : String(error);
    } finally {
      ruleSaving = false;
    }
  }

  function confirmRuleRemoval(rule: XBPSConfigRuleView) {
    openConfirmation(
      'Remove XBPS rule',
      `Remove ${rule.kind.toLowerCase()} ${rule.value} from Sysentra-managed configuration.`,
      true,
      '',
      async () => { xbpsConfigRules = await ChangeXBPSRule('remove', ruleKindID(rule), rule.value, ''); }
    );
  }

  function scheduleSearch(value: string) {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => globalQuery = value, 120);
  }

  function applyColorMode() {
    darkMode = colorMode === 'dark' || (colorMode === 'auto' && systemColorMode.matches);
    document.documentElement.dataset.mode = darkMode ? 'dark' : 'light';
    document.documentElement.style.colorScheme = darkMode ? 'dark' : 'light';
  }

  function setColorMode(mode: 'auto' | 'light' | 'dark', persist = true) {
    colorMode = mode;
    applyColorMode();
    if (persist) {
      try { localStorage.setItem('sysentra-color-mode', mode); } catch { /* appearance still applies for this session */ }
    }
  }

  function cycleColorMode() {
    setColorMode(colorMode === 'auto' ? 'light' : colorMode === 'light' ? 'dark' : 'auto');
  }

  function applyAppearance(persist = true) {
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.fontFamily = font === 'system' ? 'ui-sans-serif, system-ui, sans-serif' : `${JSON.stringify(font)}, ui-sans-serif, system-ui, sans-serif`;
    document.documentElement.style.fontSize = `${fontSize}px`;
    if (persist) {
      try {
        localStorage.setItem('sysentra-theme', theme);
        localStorage.setItem('sysentra-font', font);
        localStorage.setItem('sysentra-font-size', String(fontSize));
      } catch { /* appearance still applies for this session */ }
    }
  }

  function setFontSize(value: number) {
    fontSize = Math.min(24, Math.max(10, Math.round(Number.isFinite(value) ? value : 16)));
    applyAppearance();
  }

  async function restoreAdditionalSources() {
    try {
      const savedFlatpak = localStorage.getItem('sysentra-source-flatpak');
      additionalSources = savedFlatpak === 'false' ? await SetAdditionalSourceEnabled('flatpak', false) : await ListAdditionalSources();
    } catch {
      additionalSources = await ListAdditionalSources().catch(() => []);
    } finally {
      loadPackages();
      refreshUpdateCount();
    }
  }

  async function openSettings() {
    settingsError = '';
    await Promise.all([ListAdditionalSources().then((sources) => additionalSources = sources).catch(() => {}), loadSystemFonts()]);
    settingsDialog.showModal();
  }

  async function loadSystemFonts() {
    try {
      systemFonts = await ListSystemFonts();
      if (font !== 'system' && !systemFonts.includes(font)) {
        font = 'system';
        applyAppearance();
      }
    } catch { /* the system font remains available */ }
  }

  async function toggleAdditionalSource(source: AdditionalSourceView) {
    sourceSaving = source.id;
    settingsError = '';
    try {
      additionalSources = await SetAdditionalSourceEnabled(source.id, !source.enabled);
      try { localStorage.setItem(`sysentra-source-${source.id}`, String(!source.enabled)); } catch { /* source still applies for this session */ }
      invalidatePackageCache();
      if (activeView === 'Discover' || activeView === 'Installed' || activeView === 'Updates') loadPackages();
      refreshUpdateCount();
    } catch (error) {
      settingsError = error instanceof Error ? error.message : String(error);
    } finally {
      sourceSaving = '';
    }
  }

  function openExternalURL(value: string) {
    try {
      const url = new URL(value);
      if (url.protocol === 'https:' || url.protocol === 'http:') void Browser.OpenURL(url.toString());
    } catch { /* invalid external URL stays inert */ }
  }

  function formatBytes(value: number) {
    if (!Number.isFinite(value) || value <= 0) return '—';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const unit = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1);
    return `${Math.round(value / (1024 ** unit)).toLocaleString()}${units[unit]}`;
  }

  async function openTransactionHistory() {
    historyError = '';
    try {
      transactionHistory = await ListTransactionHistory();
    } catch (error) {
      historyError = error instanceof Error ? error.message : String(error);
    }
    historyDialog.showModal();
  }

  function confirmRepositoryToggle(repository: RepositoryView) {
    const action = repository.enabled ? 'Disable' : 'Enable';
    openConfirmation(`${action} repository`, `${action} ${repository.url}. Disabling will comment its repository entry; enabling will restore it.`, false, '', async () => { repositories = await ChangeRepository(action.toLowerCase(), repository.url, '', false); });
  }

  function sambaPrincipals(value: string, groupsOnly: boolean) {
    return value.split(',').map((part) => part.trim()).filter((part) => part && (groupsOnly ? part.startsWith('@') : !part.startsWith('@'))).map((part) => groupsOnly ? part.slice(1) : part).join(', ');
  }

  function sambaAccessList(usersList: string, selectedGroups: string[]) {
    return [...usersList.split(',').map((part) => part.trim()).filter(Boolean), ...selectedGroups.map((group) => `@${group}`)].join(', ');
  }

  function toggleSambaGroup(target: 'valid' | 'write' | 'read', group: string) {
    const current = target === 'valid' ? configValidGroups : target === 'write' ? configWriteGroups : configReadGroups;
    const updated = current.includes(group) ? current.filter((item) => item !== group) : [...current, group];
    if (target === 'valid') configValidGroups = updated;
    if (target === 'write') configWriteGroups = updated;
    if (target === 'read') configReadGroups = updated;
  }

  function openSambaGlobalDialog() {
    const global = sambaServer?.global;
    sambaGlobalProtocol = global?.protocol || 'SMB2';
    sambaGlobalMask = global?.defaultMask || '0755';
    sambaGlobalPrinting = global?.printing ?? false;
    sambaGlobalMapGuest = global?.mapGuest ?? false;
    sambaGlobalFirewallAllowed = global?.firewallAllowed ?? false;
    sambaGlobalError = '';
    sambaGlobalDialog.showModal();
  }

  async function saveSambaGlobal() {
    if (sambaGlobalSaving) return;
    sambaGlobalSaving = true;
    sambaGlobalError = '';
    try {
      sambaServer = await ChangeSambaGlobal({ protocol: sambaGlobalProtocol, defaultMask: sambaGlobalMask, printing: sambaGlobalPrinting, mapGuest: sambaGlobalMapGuest, firewallAllowed: sambaGlobalFirewallAllowed });
      sambaGlobalDialog.close();
    } catch (error) {
      sambaGlobalError = error instanceof Error ? error.message : String(error);
    } finally {
      sambaGlobalSaving = false;
    }
  }

  async function openConfigDialog(kind: 'repository' | 'flatpak' | 'samba' | 'nfs', operation = 'add', item: RepositoryView | FlatpakRemoteView | SambaShareView | NFSMountView | null = null) {
    configKind = kind;
    configOperation = operation;
    configValue = kind === 'samba' && item ? (item as SambaShareView).name : kind === 'nfs' && item ? (item as NFSMountView).source : (item as RepositoryView | FlatpakRemoteView | null)?.url ?? '';
    configOldValue = configValue;
    configName = kind === 'flatpak' && item ? (item as FlatpakRemoteView).name : '';
    configPath = kind === 'samba' && item ? (item as SambaShareView).path : kind === 'nfs' && item ? (item as NFSMountView).target : '';
    configOldPath = configPath;
    configType = kind === 'nfs' && item ? (item as NFSMountView).type : 'nfs4';
    configOptions = kind === 'nfs' && item ? (item as NFSMountView).options : 'rw,_netdev';
    configReadOnly = kind === 'samba' && item ? (item as SambaShareView).readOnly : true;
    configGuestOK = kind === 'samba' && item ? (item as SambaShareView).guestOK : false;
    const share = kind === 'samba' && item ? item as SambaShareView : null;
    configComment = share?.comment ?? '';
    configBrowseable = share?.browseable ?? true;
    configFollowSymlinks = share?.followSymlinks ?? true;
    configWideLinks = share?.wideLinks ?? false;
    configValidUsers = sambaPrincipals(share?.validUsers ?? '', false);
    configWriteList = sambaPrincipals(share?.writeList ?? '', false);
    configReadList = sambaPrincipals(share?.readList ?? '', false);
    configValidGroups = sambaPrincipals(share?.validUsers ?? '', true).split(',').map((group) => group.trim()).filter(Boolean);
    configWriteGroups = sambaPrincipals(share?.writeList ?? '', true).split(',').map((group) => group.trim()).filter(Boolean);
    configReadGroups = sambaPrincipals(share?.readList ?? '', true).split(',').map((group) => group.trim()).filter(Boolean);
    configForceUser = share?.forceUser ?? '';
    configForceGroup = share?.forceGroup ?? '';
    configCreateMask = share?.createMask || '0744';
    configDirectoryMask = share?.directoryMask || '0755';
    configError = '';
    configTrustConfirmed = false;
    configRepositoryKey = null;
    configKeyURL = '';
    configKeyLoading = false;
    configDialog.showModal();
    if (kind === 'samba' && groups.length === 0) {
      try {
        const result = await ListAccounts();
        users = result.users;
        groups = result.groups;
      } catch (error) {
        configError = `Could not load local users and groups: ${error instanceof Error ? error.message : String(error)}`;
      }
    }
  }

  async function saveConfig() {
    if (configSaving) return;
    configSaving = true;
    configError = '';
    try {
      if (configKind === 'repository') repositories = await ChangeRepository(configOperation, configValue.trim(), configOldValue, configTrustConfirmed);
      if (configKind === 'flatpak') flatpakRemotes = await ChangeFlatpakRemote(configOperation, configName.trim(), configValue.trim());
      if (configKind === 'samba') sambaServer = await ChangeSambaShare({ operation: configOperation, name: configValue.trim(), oldName: configOldValue, path: configPath.trim(), comment: configComment.trim(), browseable: configBrowseable, readOnly: configReadOnly, guestOK: configGuestOK, followSymlinks: configFollowSymlinks, wideLinks: configWideLinks, validUsers: sambaAccessList(configValidUsers, configValidGroups), writeList: sambaAccessList(configWriteList, configWriteGroups), readList: sambaAccessList(configReadList, configReadGroups), forceUser: configForceUser.trim(), forceGroup: configForceGroup.trim(), createMask: configCreateMask, directoryMask: configDirectoryMask });
      if (configKind === 'nfs') nfsMounts = await ChangeNFSMount(configOperation, configValue.trim(), configOldValue, configPath.trim(), configOldPath, configType, configOptions.trim());
      configDialog.close();
    } catch (error) {
      configError = error instanceof Error ? error.message : String(error);
    } finally {
      configSaving = false;
    }
  }

  function confirmSambaRemoval(share: SambaShareView) {
    openConfirmation('Remove Samba share', `Remove ${share.name} from Samba configuration without deleting shared files.`, true, '', async () => {
      sambaServer = await ChangeSambaShare({ operation: 'remove', name: share.name, oldName: '', path: '', comment: '', browseable: false, readOnly: false, guestOK: false, followSymlinks: false, wideLinks: false, validUsers: '', writeList: '', readList: '', forceUser: '', forceGroup: '', createMask: '', directoryMask: '' });
    });
  }

  function confirmNFSAction(operation: 'mount' | 'unmount' | 'remove', mount: NFSMountView) {
    const action = operation === 'unmount' ? 'Unmount' : operation === 'mount' ? 'Mount' : 'Remove';
    const description = operation === 'remove' ? `Remove ${mount.source} from persistent configuration without deleting local files.` : `${action} ${mount.source} at ${mount.target}.`;
    openConfirmation(`${action} NFS mount`, description, operation !== 'mount', '', async () => {
      nfsMounts = await ChangeNFSMount(operation, mount.source, '', mount.target, '', mount.type, mount.options);
    });
  }

  function openFirmwareAction(operation: 'refresh' | 'update', device: FirmwareDeviceView | null = null) {
    firmwareOperation = operation;
    firmwareDevice = device;
    firmwareActionLoading = false;
    firmwareActionError = '';
    firmwareActionLogs = [];
    firmwareActionSucceeded = false;
    firmwareDialog.showModal();
  }

  async function executeFirmwareAction() {
    if (firmwareActionLoading || (firmwareOperation === 'update' && !firmwareDevice?.newVersion)) return;
    firmwareActionLoading = true;
    firmwareActionError = '';
    firmwareActionLogs = [];
    try {
      const result = await ChangeFirmware(firmwareOperation, firmwareDevice?.id ?? '', firmwareDevice?.newVersion ?? '');
      firmwareAvailable = result.catalog.available;
      firmwareDevices = result.catalog.devices;
      firmwareWarning = result.catalog.warning;
      firmwareActionLogs = result.diagnostics;
      firmwareActionSucceeded = true;
    } catch (error) {
      firmwareActionError = error instanceof Error ? error.message : String(error);
    } finally {
      firmwareActionLoading = false;
    }
  }

  async function inspectRepositoryKey() {
    const repositoryURL = configValue.trim();
    if (!repositoryURL || configKeyLoading) return;
    configKeyLoading = true;
    configError = '';
    configTrustConfirmed = false;
    configRepositoryKey = null;
    configKeyURL = '';
    try {
      configRepositoryKey = await InspectRepositoryKey(repositoryURL);
      configKeyURL = repositoryURL;
    } catch (error) {
      configError = error instanceof Error ? error.message : String(error);
    } finally {
      configKeyLoading = false;
    }
  }

  function openAccountDialog(operation: string, user = '', group = '') {
    accountOperation = operation;
    accountUser = user;
    accountGroup = group;
    accountGroups = [];
    accountCreateHome = true;
    accountHome = '';
    accountShell = '';
    accountMoveHome = false;
    if (user && (operation === 'user-edit' || operation === 'primary-group-change')) {
      const current = users.find((item) => item.name === user);
      accountHome = current?.home ?? '';
      accountShell = current?.shell ?? '';
      accountGroup = operation === 'primary-group-change' ? current?.primaryGroup ?? '' : group;
    }
    accountError = '';
    accountDialog.showModal();
  }

  async function saveAccount() {
    if (accountSaving) return;
    accountSaving = true;
    accountError = '';
    try {
      const result = accountOperation === 'user-add' ? await CreateUser(accountUser.trim(), accountCreateHome, accountCreateHome ? accountHome.trim() : '') : accountOperation === 'membership-add' ? await AssignAccountGroups(accountUser.trim(), accountGroups) : accountOperation === 'user-edit' ? await EditUser(accountUser.trim(), accountHome.trim(), accountShell.trim(), accountMoveHome) : await ChangeAccount(accountOperation, accountUser.trim(), accountGroup.trim());
      users = result.users;
      groups = result.groups;
      accountDialog.close();
    } catch (error) {
      accountError = error instanceof Error ? error.message : String(error);
    } finally {
      accountSaving = false;
    }
  }

  function toggleAccountGroup(group: string) {
    accountGroups = accountGroups.includes(group) ? accountGroups.filter((item) => item !== group) : [...accountGroups, group];
  }

  function confirmAccountRemoval(kind: 'user' | 'group', name: string) {
    const user = kind === 'user' ? users.find((item) => item.name === name) : null;
    const activity = user && (user.sessions || user.processes) ? ` ${user.sessions} active session(s) and ${user.processes} running process(es) were detected.` : '';
    openConfirmation(`Remove ${kind}`, `Remove ${kind} ${name}. User home directories and files are preserved.${activity}`, true, '', async () => {
      const result = await ChangeAccount(`${kind}-remove`, kind === 'user' ? name : '', kind === 'group' ? name : '');
      users = result.users;
      groups = result.groups;
    });
  }

  function confirmPasswordLock(user: UserView) {
    const unlocking = user.password === 'Locked';
    openConfirmation(`${unlocking ? 'Unlock' : 'Lock'} ${user.name}`, `${unlocking ? 'Unlock' : 'Lock'} password authentication for ${user.name}.`, !unlocking, '', async () => {
      const result = await ChangeAccount(unlocking ? 'password-unlock' : 'password-lock', user.name, '');
      users = result.users;
      groups = result.groups;
    });
  }

  function runAccountAction(event: MouseEvent, action: () => void) {
    const menu = (event.currentTarget as HTMLElement).closest('details');
    if (menu) menu.open = false;
    action();
  }

  async function openAccountHistory() {
    try {
      accountHistory = await ListAccountHistory();
      accountHistoryDialog.showModal();
    } catch (error) {
      systemError = error instanceof Error ? error.message : String(error);
    }
  }

  function confirmRepositoryRemoval(repository: RepositoryView) {
    openConfirmation('Remove repository', `Remove ${repository.url} from Sysentra-managed XBPS configuration. Installed packages are not removed.`, true, '', async () => { repositories = await ChangeRepository('remove', repository.url, '', false); });
  }

  function confirmFlatpakRemoteToggle(remote: FlatpakRemoteView) {
    const operation = remote.enabled ? 'disable' : 'enable';
    openConfirmation(`${remote.enabled ? 'Disable' : 'Enable'} Flatpak remote`, `${remote.enabled ? 'Disable' : 'Enable'} ${remote.name}.`, false, '', async () => { flatpakRemotes = await ChangeFlatpakRemote(operation, remote.name, ''); });
  }

  function confirmFlatpakRemoteRemoval(remote: FlatpakRemoteView) {
    openConfirmation('Remove Flatpak remote', `Remove ${remote.name}. Installed Flatpak applications are not removed.`, true, '', async () => { flatpakRemotes = await ChangeFlatpakRemote('remove', remote.name, ''); });
  }

  function confirmMirror(value: string) {
    const resetting = value === '';
    openConfirmation(resetting ? 'Reset default mirror' : 'Change Void mirror', resetting ? 'Restore repo-default.voidlinux.org for every Void vendor repository, then refresh XBPS indexes.' : `Use ${value} for every Void vendor repository, then refresh XBPS indexes.`, false, '', async () => {
      const result = await ChangeMirror(value);
      mirrors = result.mirrors ?? [];
      mirrorWarnings = result.warnings ?? [];
    });
  }

  function confirmFirewallToggle(state: FirewallView | null) {
    if (!state) return;
    const operation = state.active ? 'disable' : 'enable';
    const action = state.active ? 'Disable' : 'Enable';
    openConfirmation(`${action} firewall`, `${action} ${state.backend}.`, state.active, '', async () => {
      storeFirewall(await ChangeFirewall(firewallChange(operation, state.backend)));
    });
  }

  async function selectFirewallBackend(backend: string) {
    const requestID = ++firewallRequestID;
    firewallSelectedBackend = backend;
    firewallContentError = '';
    if (firewallCache[backend]) {
      firewall = firewallCache[backend];
      firewallContentLoading = false;
      return;
    }
    firewallContentLoading = true;
    try {
      const result = await GetFirewall(backend);
      if (requestID === firewallRequestID) storeFirewall(result);
    } catch (error) {
      if (requestID === firewallRequestID) firewallContentError = error instanceof Error ? error.message : String(error);
    } finally {
      if (requestID === firewallRequestID) firewallContentLoading = false;
    }
  }

  async function reloadFirewallStatus() {
    const backend = firewallSelectedBackend || firewall?.backend || '';
    const requestID = ++firewallRequestID;
    firewallContentLoading = true;
    firewallContentError = '';
    try {
      const result = await GetFirewall(backend);
      if (requestID === firewallRequestID) storeFirewall(result);
    } catch (error) {
      if (requestID === firewallRequestID) firewallContentError = error instanceof Error ? error.message : String(error);
    } finally {
      if (requestID === firewallRequestID) firewallContentLoading = false;
    }
  }

  function storeFirewall(result: FirewallView, invalidate = false) {
    firewallCache = invalidate ? { [result.backend]: result } : { ...firewallCache, [result.backend]: result };
    firewall = result;
    firewallSelectedBackend = result.backend;
  }

  function confirmFirewallSelection() {
    if (!firewall) return;
    openConfirmation(`Use ${firewall.backend}`, `Disable any other active firewall backend, then enable ${firewall.backend}.`, false, '', async () => {
      storeFirewall(await ChangeFirewall(firewallChange('select', firewall!.backend)), true);
    });
  }

  function openFirewallDialog(mode: typeof firewallDialogMode, rule: FirewallRuleView | null = null) {
    if (!firewall) return;
    firewallDialogMode = mode;
    firewallOldRule = rule;
    firewallRuleKind = rule?.kind === 'service' ? 'service' : 'port';
    firewallTarget = rule?.target ?? '';
    firewallProtocol = rule?.protocol || 'tcp';
    firewallRuleDirection = rule?.direction || 'in';
    firewallSource = rule?.source && rule.source !== 'Anywhere' ? rule.source : 'any';
    firewallAction = rule?.action || 'allow';
    firewallPolicy = 'deny';
    firewallDirection = 'incoming';
    firewallZone = firewall.zone || '';
    firewallError = '';
    firewallDialog.showModal();
  }

  function firewallRule() {
    return { kind: firewallRuleKind, target: firewallTarget.trim(), protocol: firewallRuleKind === 'port' ? firewallProtocol : '', direction: firewall!.backend === 'ufw' ? firewallRuleDirection : '', source: firewall!.backend === 'ufw' ? firewallSource.trim() || 'any' : 'any', action: firewall!.backend === 'ufw' ? firewallAction : 'allow', mutable: true };
  }

  function emptyFirewallRule(): FirewallRuleView { return { kind: '', target: '', protocol: '', direction: '', source: '', action: '', mutable: false }; }

  function firewallChange(operation: string, backend: string, fields: Partial<FirewallChangeInput> = {}): FirewallChangeInput {
    return { operation, backend, zone: '', policy: '', direction: '', rule: emptyFirewallRule(), oldRule: emptyFirewallRule(), ...fields };
  }

  function saveFirewallConfiguration() {
    if (!firewall) return;
    firewallError = '';
    const input = firewallDialogMode === 'policy'
      ? firewallChange('policy-set', firewall.backend, { policy: firewallPolicy, direction: firewallDirection })
      : firewallDialogMode === 'zone'
        ? firewallChange('zone-set', firewall.backend, { zone: firewallZone.trim() })
        : firewallChange(firewallDialogMode === 'edit' ? 'rule-edit' : 'rule-add', firewall.backend, { zone: firewall.zone, rule: firewallRule(), oldRule: firewallOldRule ?? emptyFirewallRule() });
    const title = firewallDialogMode === 'policy' ? 'Change default policy' : firewallDialogMode === 'zone' ? 'Change default zone' : firewallDialogMode === 'edit' ? 'Edit firewall rule' : 'Add firewall rule';
    const description = firewallDialogMode === 'policy' ? `Set ${firewallDirection} traffic to ${firewallPolicy} in UFW.` : firewallDialogMode === 'zone' ? `Set ${firewallZone.trim()} as the firewalld default zone after verifying SSH access.` : `${firewallDialogMode === 'edit' ? 'Replace' : 'Add'} ${firewallRuleKind === 'port' ? `${firewallTarget}/${firewallProtocol}` : firewallTarget} in ${firewall.backend}.`;
    firewallDialog.close();
    openConfirmation(title, description, firewallDialogMode !== 'add', '', async () => { storeFirewall(await ChangeFirewall(input)); });
  }

  function confirmFirewallRuleRemoval(rule: FirewallRuleView) {
    if (!firewall) return;
    openConfirmation('Remove firewall rule', `Remove ${rule.kind === 'port' ? `${rule.target}/${rule.protocol}` : rule.target} from ${firewall.backend}. SSH access rules cannot be removed.`, true, '', async () => {
      storeFirewall(await ChangeFirewall(firewallChange('rule-remove', firewall!.backend, { zone: firewall!.zone, rule })));
    });
  }

  function openOwnerDialog() {
    ownerQuery = '';
    ownerMatches = [];
    ownerError = '';
    ownerSearched = false;
    ownerDialog.showModal();
  }

  async function findOwningPackage() {
    ownerError = '';
    ownerMatches = [];
    ownerLoading = true;
    ownerSearched = false;
    try {
      ownerMatches = (await FindOwningPackage(ownerQuery)).matches;
    } catch (error) {
      ownerError = error instanceof Error ? error.message : String(error);
    } finally {
      ownerLoading = false;
      ownerSearched = true;
    }
  }

  async function toggleMirrorRecovery() {
    const enabled = !mirrorRecoveryEnabled;
    mirrorRecoverySaving = true;
    mirrorRecoveryError = '';
    try {
      mirrorRecoveryEnabled = (await SetMirrorRecovery(enabled)).enabled;
    } catch (error) {
      mirrorRecoveryError = error instanceof Error ? error.message : String(error);
    } finally {
      mirrorRecoverySaving = false;
    }
  }
</script>

<svelte:head>
  <meta name="description" content="Software and system management for Void Linux" />
</svelte:head>

<div class="grid h-full min-h-0 grid-cols-[13rem_minmax(0,1fr)] grid-rows-[2.5rem_minmax(0,1fr)_1.75rem] bg-surface-50-900 text-surface-950-50 max-[760px]:grid-cols-1 max-[760px]:grid-rows-[2.75rem_auto_minmax(0,1fr)_1.75rem]">
  <header class="app-chrome col-span-2 flex items-center gap-3 border-b border-surface-300-700 bg-surface-100-800 px-3 max-[760px]:col-span-1" aria-label="Application bar">
    <img class="size-7 object-contain" src={logoUrl} alt="" aria-hidden="true" />
    <p class="text-sm font-semibold tracking-tight">Sysentra</p>
    <label class="mx-auto w-full max-w-xl" aria-label={`Search ${activeView.toLowerCase()}`}>
      <input class="input app-input-surface h-8 w-full py-1 text-sm" type="search" placeholder="Search..." bind:value={searchInput} on:input={(event) => scheduleSearch(event.currentTarget.value)} />
    </label>
    <button type="button" aria-label={`Appearance: ${colorMode}. Activate to change mode.`} title={`Appearance: ${colorMode}`} class="ml-auto grid size-8 shrink-0 place-items-center rounded-base border border-surface-300-700 bg-surface-50-900 hover:bg-surface-200-800" on:click={cycleColorMode}>{#if colorMode === 'auto'}<svg viewBox="0 0 20 20" class="size-4" aria-hidden="true"><circle cx="10" cy="10" r="6.5" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M10 3.5a6.5 6.5 0 0 1 0 13Z" fill="currentColor"/></svg>{:else if colorMode === 'dark'}<svg viewBox="0 0 20 20" class="size-4" aria-hidden="true"><path d="M14.8 13.2A6.3 6.3 0 0 1 6.8 5.2 6.5 6.5 0 1 0 14.8 13.2Z" fill="currentColor"/></svg>{:else}<svg viewBox="0 0 20 20" class="size-4" aria-hidden="true"><circle cx="10" cy="10" r="3.25" fill="currentColor"/><path d="M10 2v2M10 16v2M2 10h2M16 10h2M4.35 4.35l1.4 1.4M14.25 14.25l1.4 1.4M15.65 4.35l-1.4 1.4M5.75 14.25l-1.4 1.4" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>{/if}</button>
  </header>

  <aside class="app-chrome flex min-h-0 flex-col border-r border-surface-300-700 bg-surface-100-800 p-2 max-[760px]:hidden" aria-label="Primary navigation">
    <nav class="flex flex-col gap-1">
      {#each navigation as item}
        <button
          type="button"
          class:font-semibold={item === activeView}
          class="rounded-base border border-transparent px-2.5 py-1.5 text-left text-sm hover:bg-surface-200-800 {item === activeView ? 'border-primary-500/30 bg-primary-500/10 text-primary-700-300' : ''}"
          aria-current={item === activeView ? 'page' : undefined}
          on:click={() => selectView(item)}
        >
          <span class="flex items-center gap-2"><NavigationIcon name={item} /><span>{item}</span>{#if item === 'Updates' && updateCount > 0}<span class="ml-auto rounded-full bg-warning-500/15 px-1.5 py-0.5 text-[0.625rem] font-semibold text-warning-700-300">{updateCount}</span>{/if}</span>
        </button>
      {/each}
    </nav>

    <div class="mt-auto border-t border-surface-300-700 pt-2">
      <button type="button" class="flex w-full items-center gap-2 rounded-base px-2.5 py-1.5 text-left text-sm hover:bg-surface-200-800" on:click={openSettings}><NavigationIcon name="Settings" />Settings</button>
    </div>
  </aside>

  <nav class="app-chrome hidden gap-1 overflow-x-auto border-b border-surface-300-700 bg-surface-100-800 p-2 max-[760px]:flex" aria-label="Primary navigation">
    {#each navigation as item}
      <button type="button" class="flex shrink-0 items-center gap-1.5 rounded-base px-3 py-1.5 text-sm {item === activeView ? 'bg-primary-500/10 font-semibold text-primary-700-300' : ''}" aria-current={item === activeView ? 'page' : undefined} on:click={() => selectView(item)}><NavigationIcon name={item} />{item}{#if item === 'Updates' && updateCount > 0}<span class="rounded-full bg-warning-500/15 px-1.5 py-0.5 text-[0.625rem] font-semibold text-warning-700-300">{updateCount}</span>{/if}</button>
    {/each}
    <button type="button" class="flex shrink-0 items-center gap-1.5 rounded-base px-3 py-1.5 text-sm hover:bg-surface-200-800" on:click={openSettings}><NavigationIcon name="Settings" />Settings</button>
  </nav>

  <main class="main-workspace min-h-0 overflow-hidden p-4 max-[760px]:p-3">
    <div class="flex h-full min-h-0 w-full flex-col gap-4 overflow-auto">
      {#if activeView === 'Discover' || activeView === 'Installed' || activeView === 'Updates'}
        <div class="flex min-h-0 flex-1 flex-col gap-4">
        {#each catalogWarnings as warning}
          <p class="rounded-base border border-warning-500/40 bg-warning-500/10 px-3 py-2 text-sm" role="status">{warning}</p>
        {/each}

        {#if activeView === 'Installed'}
          <details class="package-maintenance rounded-base border border-surface-300-700 bg-surface-100-800">
            <summary class="cursor-pointer px-3 py-2 text-sm font-semibold">Package maintenance</summary>
            <div class="maintenance-grid grid gap-2 border-t border-surface-300-700 p-3">
              {#each [
                ['Verify package database', 'Run XBPS package database consistency checks'],
                ['Reconfigure packages', 'Run xbps-reconfigure -fa after review and authorization'],
                ['Find owning package', 'Find which installed package owns a file path or matching pattern'],
                ['Remove orphans', 'Review automatically installed packages no longer required'],
                ['Clean package cache', 'Review cached package archives before deletion'],
                ['Clean unused Flatpaks', 'Remove unused user-scoped Flatpak runtimes and extensions'],
              ] as tool}
                {#if tool[0] !== 'Clean unused Flatpaks' || flatpakSourceEnabled}
                <button type="button" class="rounded-base border p-3 text-left hover:bg-surface-200-800 {tool[0] === 'Remove orphans' || tool[0] === 'Clean package cache' || tool[0] === 'Clean unused Flatpaks' ? 'border-error-500/50 text-error-700-300' : 'border-surface-300-700'}" on:click={() => openMaintenanceTool(tool[0], tool[1])}>
                  <span class="block text-sm font-medium">{tool[0]}</span><span class="block text-xs text-surface-500">{tool[1]}</span>
                </button>
                {/if}
              {/each}
            </div>
          </details>
        {/if}

        {#if packageError}
        <section class="rounded-base border border-error-500/40 bg-error-500/10 p-4" role="alert">
          <h2 class="font-semibold">Unable to load packages</h2>
          <p class="mt-1 text-sm">{packageError}</p>
        </section>
      {:else if packagesLoading}
        <section class="rounded-base border border-surface-300-700 bg-surface-100-800 p-4 text-sm" aria-live="polite" aria-busy="true">Loading packages…</section>
      {:else}
        <div class="package-workspace relative flex min-h-0 flex-1 gap-3">
          <VirtualPackageTable
            {packages}
            title={activeView === 'Discover' ? 'All packages' : activeView === 'Updates' ? 'Package updates' : 'Installed packages'}
            description={activeView === 'Discover' ? `${flatpakSourceEnabled ? 'XBPS and Flatpak' : 'XBPS'} packages from enabled sources` : activeView === 'Updates' ? 'Installed packages with newer backend versions' : `Packages reported by local ${flatpakSourceEnabled ? 'XBPS and Flatpak databases' : 'XBPS database'}`}
            action={activeView === 'Discover' ? 'install' : activeView === 'Updates' ? 'update' : 'remove'}
            showStateFilters={activeView === 'Discover' || activeView === 'Installed'}
            showSourceFilter={activeView !== 'Updates'}
            query={globalQuery}
            selectedPackageKey={selectedPackage ? `${selectedPackage.backendId}:${selectedPackage.id}` : ''}
            bind:selectedItems={selectedPackages}
            bind:resultCount={packageResultCount}
            onPackageSelect={openPackageDetails}
            onExecuteSelection={openSelectedPackageTransaction}
          >
            <div slot="actions" class="flex gap-2">
              {#if activeView === 'Updates'}<button type="button" class="btn preset-filled-primary-500" on:click={openSystemUpgrade}>Full system upgrade</button>{/if}
              <button type="button" class="btn preset-tonal" on:click={openDatabaseSync} disabled={packagesLoading}>Sync database</button>{#if activeView !== 'Discover'}<button type="button" class="btn preset-tonal" on:click={openTransactionHistory}>History</button>{/if}<button type="button" class="btn preset-tonal" on:click={reloadPackages} disabled={packagesLoading}>Reload</button>
            </div>
          </VirtualPackageTable>
          {#if selectedPackage}<PackageDetailsSidebar selected={selectedPackage} details={packageDetails} loading={packageDetailsLoading} error={packageDetailsError} onClose={closePackageDetails} onAction={(title, description, destructive, operation, targetVersion) => openConfirmation(title, description, destructive, operation, null, targetVersion)} />{/if}
        </div>
        {/if}
        </div>
      {:else if systemError}
        <p class="rounded-base border border-error-500/40 bg-error-500/10 p-4" role="alert">{systemError}</p>
      {:else if systemLoading}
        <p class="rounded-base border border-surface-300-700 bg-surface-100-800 p-4" aria-live="polite">Loading {activeView.toLowerCase()}…</p>
      {:else if activeView === 'Services'}
        <div class="flex min-h-0 flex-1 gap-3"><section class="system-table min-w-0 flex-1 rounded-base border border-surface-300-700 bg-surface-100-800" aria-labelledby="services-heading">
          <header class="flex flex-wrap items-center gap-3 border-b border-surface-300-700 p-3"><div><h2 id="services-heading" class="font-semibold">Runit services</h2><p class="text-xs text-surface-500">System actions use polkit; user and turnstile services use session permissions.</p></div><div class="ml-auto flex flex-wrap items-center gap-2">{#if serviceScope !== 'turnstile'}<button type="button" class="btn preset-filled-primary-500" on:click={openServiceDialog}>Add service</button>{/if}{#if serviceScope === 'user'}<button type="button" class="btn preset-tonal" on:click={initializeUserServices}>Initialize</button>{/if}<button type="button" class="btn preset-tonal" on:click={() => loadSystemView()} disabled={systemLoading}>Reload</button></div></header>
          <div class="flex flex-wrap items-end gap-1 border-b border-surface-300-700 bg-surface-50-900 px-3 pt-2"><div class="flex gap-1" role="tablist" aria-label="Service scope"><button type="button" role="tab" aria-selected={serviceScope === 'system'} class="border-b-2 px-3 py-2 text-sm {serviceScope === 'system' ? 'border-primary-500 font-semibold text-primary-700-300' : 'border-transparent text-surface-500 hover:text-surface-900-100'}" on:click={() => selectServiceScope('system')}>System</button><button type="button" role="tab" aria-selected={serviceScope === 'user'} class="border-b-2 px-3 py-2 text-sm {serviceScope === 'user' ? 'border-primary-500 font-semibold text-primary-700-300' : 'border-transparent text-surface-500 hover:text-surface-900-100'}" on:click={() => selectServiceScope('user')}>User</button><button type="button" role="tab" aria-selected={serviceScope === 'turnstile'} class="border-b-2 px-3 py-2 text-sm {serviceScope === 'turnstile' ? 'border-primary-500 font-semibold text-primary-700-300' : 'border-transparent text-surface-500 hover:text-surface-900-100'}" on:click={() => selectServiceScope('turnstile')}>Turnstile</button></div><div class="ml-auto flex gap-1 pb-1.5" aria-label="Service filter">{#each [['all', 'All'], ['active', 'Active'], ['disabled', 'Disabled']] as filter}<button type="button" class="rounded-base border px-2.5 py-1 text-xs {serviceFilter === filter[0] ? 'border-primary-500 bg-primary-500/10 font-semibold text-primary-700-300' : 'border-surface-300-700 hover:bg-surface-200-800'}" aria-pressed={serviceFilter === filter[0]} on:click={() => serviceFilter = filter[0] as typeof serviceFilter}>{filter[1]}</button>{/each}</div></div>
          {#if serviceScope === 'turnstile'}<div class="grid gap-2 border-b border-surface-300-700 p-3 md:grid-cols-2"><div class="flex gap-2"><label class="sr-only" for="turnstile-ready">Turnstile ready services</label><input id="turnstile-ready" class="input min-w-0 flex-1" placeholder="Ready services…" bind:value={turnstileReady} /><button type="button" class="btn preset-tonal" on:click={() => changeTurnstileReady('add')}>Add</button><button type="button" class="btn preset-tonal" on:click={() => changeTurnstileReady('remove')}>Remove</button></div><div class="flex gap-2"><label class="sr-only" for="turnstile-environment">Turnstile environment variables</label><input id="turnstile-environment" class="input min-w-0 flex-1" placeholder="VAR=value, OTHER=value" bind:value={turnstileEnvironment} /><button type="button" class="btn preset-tonal" on:click={updateTurnstileEnvironment}>Update environment</button></div></div>{/if}
          <div class="service-grid package-table-header grid border-b border-surface-300-700 px-3 py-2 text-xs font-semibold"><span>Service</span><span>State</span><span>Enabled</span><span>PID</span><span>Command</span><span>Time</span></div>
          <div class="system-table-body">{#each filteredServices as service}
            <div class="service-grid grid cursor-pointer items-center border-b border-surface-200-800 px-3 py-2 text-sm hover:bg-primary-500/5 {selectedService?.name === service.name ? 'bg-primary-500/10' : ''}" role="button" aria-label={`Open ${service.name} service details`} tabindex="0" on:click={() => openServiceDetails(service)} on:keydown={(event) => { if (event.key === 'Enter' || event.key === ' ') openServiceDetails(service); }}>
              <button type="button" class="truncate text-left font-medium hover:text-primary-600" aria-pressed={selectedService?.name === service.name} on:click|stopPropagation={() => openServiceDetails(service)}>{service.name}</button>{#if !service.enabled}<span class="text-surface-500">---</span>{:else}<span class="w-fit rounded-full border px-2 py-0.5 text-xs {service.status === 'Running' ? 'border-success-500/40 bg-success-500/10 text-success-700-300' : service.status === 'Stopped' ? 'border-error-500/40 bg-error-500/10 text-error-700-300' : 'border-warning-500/40 bg-warning-500/10 text-warning-700-300'}">{service.status === 'Running' ? '✔ running' : service.status === 'Stopped' ? '■ stopped' : `? ${service.status.toLowerCase()}`}</span>{/if}
              <button type="button" role="switch" aria-checked={service.enabled} aria-label={`${service.nextAction} ${service.name}`} class="relative h-5 w-9 rounded-full {service.enabled ? 'bg-success-600' : 'bg-surface-400'}" on:click|stopPropagation={() => confirmServiceOperation(service, service.nextAction)}><span class="absolute top-0.5 size-4 rounded-full bg-white transition-[left] {service.enabled ? 'left-[1.125rem]' : 'left-0.5'}"></span></button>
              <span class="font-mono text-xs">{service.pid || '-'}</span><span class="service-command truncate font-mono text-xs" title={service.command}>{service.command || '-'}</span><span class="font-mono text-xs">{service.time || '-'}</span>
            </div>
          {/each}</div>
        </section>{#if selectedService}<ServiceDetailsSidebar selected={selectedService} details={serviceDetails} loading={serviceDetailsLoading} error={serviceDetailsError} onClose={() => { selectedService = null; serviceDetails = null; }} onReload={() => openServiceDetails(selectedService!)} onAction={(operation) => confirmServiceOperation(selectedService, operation)} />{/if}</div>
      {:else if activeView === 'Users & Groups'}
        <section class="system-table rounded-base border border-surface-300-700 bg-surface-100-800" aria-labelledby="accounts-heading">
          <header class="flex flex-wrap items-center gap-3 border-b border-surface-300-700 p-3"><div><h2 id="accounts-heading" class="font-semibold">Users &amp; Groups</h2><p class="text-xs text-surface-500">Local account, authentication and activity status.</p></div><div class="ml-auto flex gap-2"><button type="button" class="btn preset-tonal" on:click={openAccountHistory}>History</button><button type="button" class="btn preset-filled-primary-500" on:click={() => openAccountDialog(accountTab === 'users' ? 'user-add' : 'group-add')}>Add {accountTab === 'users' ? 'user' : 'group'}</button><button type="button" class="btn preset-tonal" on:click={() => loadSystemView()} disabled={systemLoading}>Reload</button></div></header>
          <div class="flex gap-1 border-b border-surface-300-700 bg-surface-50-900 px-3 pt-2" role="tablist"><button type="button" role="tab" aria-selected={accountTab === 'users'} class="border-b-2 px-3 py-2 text-sm {accountTab === 'users' ? 'border-primary-500 font-semibold text-primary-700-300' : 'border-transparent text-surface-500'}" on:click={() => accountTab = 'users'}>Users</button><button type="button" role="tab" aria-selected={accountTab === 'groups'} class="border-b-2 px-3 py-2 text-sm {accountTab === 'groups' ? 'border-primary-500 font-semibold text-primary-700-300' : 'border-transparent text-surface-500'}" on:click={() => accountTab = 'groups'}>Groups</button></div>
          {#if accountTab === 'users'}<div class="account-user-grid package-table-header grid border-b border-surface-300-700 px-3 py-2 text-xs font-semibold"><span>User</span><span>UID</span><span>Status</span><span>Activity</span><span>Home / shell</span><span>Groups</span><span>Actions</span></div><div class="system-table-body">{#each filteredUsers as user}<div class="account-user-grid grid items-center border-b border-surface-200-800 px-3 py-2 text-sm"><span class="truncate font-medium">{user.name}</span><span class="font-mono text-xs">{user.uid}</span><span class="w-fit rounded-full border px-2 py-0.5 text-xs {user.password === 'Password set' ? 'border-success-500/40 bg-success-500/10 text-success-700-300' : user.password === 'Locked' ? 'border-error-500/40 bg-error-500/10 text-error-700-300' : 'border-warning-500/40 bg-warning-500/10 text-warning-700-300'}">{user.password || 'Unknown'}</span><span class="text-xs" title={`${user.sessions} sessions, ${user.processes} processes`}>{user.sessions || user.processes ? `${user.sessions} session · ${user.processes} proc` : 'Inactive'}</span><span class="min-w-0"><span class="block truncate font-mono text-xs" title={user.home}>{user.home}</span><span class="block truncate font-mono text-xs text-surface-500" title={user.shell}>{user.shell}</span></span><span class="truncate text-xs" title={user.groups.join(', ')}><strong>{user.primaryGroup || '—'}</strong>{user.groups.filter((name) => name !== user.primaryGroup).length ? ` · ${user.groups.filter((name) => name !== user.primaryGroup).join(', ')}` : ''}</span><details class="account-actions relative"><summary class="btn preset-tonal cursor-pointer select-none list-none">Actions <span aria-hidden="true">▾</span></summary><div class="absolute right-0 z-20 mt-1 grid min-w-44 gap-1 rounded-base border border-surface-300-700 bg-surface-100-800 p-1 shadow-xl"><span title={user.removable ? '' : 'System accounts cannot be edited'}><button type="button" class="w-full rounded-base px-2 py-1.5 text-left hover:bg-surface-200-800 disabled:cursor-not-allowed disabled:opacity-50" disabled={!user.removable} on:click={(event) => runAccountAction(event, () => openAccountDialog('user-edit', user.name))}>Edit account</button></span><span title={user.removable ? '' : 'System accounts cannot change primary group'}><button type="button" class="w-full rounded-base px-2 py-1.5 text-left hover:bg-surface-200-800 disabled:cursor-not-allowed disabled:opacity-50" disabled={!user.removable} on:click={(event) => runAccountAction(event, () => openAccountDialog('primary-group-change', user.name))}>Change primary group</button></span><button type="button" class="rounded-base px-2 py-1.5 text-left hover:bg-surface-200-800" on:click={(event) => runAccountAction(event, () => openAccountDialog('membership-add', user.name))}>Assign groups</button><span title={user.groups.some((name) => groups.find((group) => group.name === name)?.gid !== user.gid) ? '' : 'No supplementary groups to unassign'}><button type="button" class="w-full rounded-base px-2 py-1.5 text-left hover:bg-surface-200-800 disabled:cursor-not-allowed disabled:opacity-50" disabled={!user.groups.some((name) => groups.find((group) => group.name === name)?.gid !== user.gid)} on:click={(event) => runAccountAction(event, () => openAccountDialog('membership-remove', user.name))}>Unassign group</button></span><span title={user.removable ? '' : 'System accounts cannot be locked or unlocked'}><button type="button" class="w-full rounded-base px-2 py-1.5 text-left hover:bg-surface-200-800 disabled:cursor-not-allowed disabled:opacity-50" disabled={!user.removable} on:click={(event) => runAccountAction(event, () => confirmPasswordLock(user))}>{user.password === 'Locked' ? 'Unlock password' : 'Lock password'}</button></span><span title={user.removable ? 'The home directory is preserved' : 'System accounts cannot be removed'}><button type="button" class="w-full rounded-base px-2 py-1.5 text-left text-error-700-300 hover:bg-error-500/10 disabled:cursor-not-allowed disabled:opacity-50" disabled={!user.removable} on:click={(event) => runAccountAction(event, () => confirmAccountRemoval('user', user.name))}>Remove user</button></span></div></details></div>{:else}<p class="p-5 text-sm text-surface-500">No matching users.</p>{/each}</div>{:else}<div class="account-group-grid package-table-header grid border-b border-surface-300-700 px-3 py-2 text-xs font-semibold"><span>Group</span><span>GID</span><span>Explicit members</span><span>Actions</span></div><div class="system-table-body">{#each filteredGroups as group}<div class="account-group-grid grid items-center border-b border-surface-200-800 px-3 py-2 text-sm"><span class="truncate font-medium">{group.name}</span><span class="font-mono text-xs">{group.gid}</span><span class="truncate text-xs" title={group.members.join(', ')}>{group.members.join(', ') || '—'}</span><span><button type="button" class="btn border-error-500/50 text-error-700-300" disabled={!group.removable} title={group.removable ? 'Remove this group' : 'System groups are protected'} on:click={() => confirmAccountRemoval('group', group.name)}>Remove</button></span></div>{:else}<p class="p-5 text-sm text-surface-500">No matching groups.</p>{/each}</div>{/if}
        </section>
      {:else if activeView === 'Firewall'}
        <section class="system-table rounded-base border border-surface-300-700 bg-surface-100-800" aria-labelledby="firewall-heading">
          <header class="flex flex-wrap items-center gap-3 border-b border-surface-300-700 p-3">
            <div><h2 id="firewall-heading" class="font-semibold">Firewall</h2><p class="text-xs text-surface-500">Manage one explicitly selected firewall backend.</p></div>
            <div class="ml-auto flex gap-2">{#if firewall?.installed}<button type="button" class="btn preset-filled-primary-500" disabled={firewallContentLoading || firewallSelectedBackend !== firewall.backend || !firewall.active} title={firewall.active ? 'Add a firewall rule' : 'Enable this backend before adding rules'} on:click={() => openFirewallDialog('add')}>Add rule</button>{/if}<button type="button" class="btn preset-tonal" on:click={reloadFirewallStatus} disabled={firewallContentLoading}>Reload</button></div>
          </header>
          {#if !firewall?.installed}
            <div class="space-y-3 p-5"><p class="text-sm text-surface-600-400" role="status">Neither UFW nor firewalld is installed.</p><div class="flex gap-2"><button type="button" class="btn preset-tonal" on:click={() => openNamedPackageTransaction('Install UFW', 'Install UFW and leave it disabled until its initial policy and rules have been reviewed.', ['ufw'], 'install')}>Install UFW</button><button type="button" class="btn preset-tonal" on:click={() => openNamedPackageTransaction('Install firewalld', 'Install firewalld and leave it disabled until its initial zone and rules have been reviewed.', ['firewalld'], 'install')}>Install firewalld</button></div></div>
          {:else}
            {#if firewall.backends.length > 1}<div class="flex items-center gap-1 border-b border-surface-300-700 bg-surface-50-900 px-3 pt-2" role="tablist" aria-label="Installed firewall backends">{#each firewall.backends as backend}<button type="button" role="tab" aria-selected={firewallSelectedBackend === backend} class="border-b-2 px-3 py-2 text-sm {firewallSelectedBackend === backend ? 'border-primary-500 font-semibold text-primary-700-300' : 'border-transparent text-surface-500'}" on:click={() => selectFirewallBackend(backend)}>{backend === 'ufw' ? 'UFW' : 'firewalld'}</button>{/each}<span class="ml-auto pb-2 text-xs text-warning-700-300">Only one backend should be active.</span></div>{/if}
            {#if firewallContentLoading}<div class="system-table-body grid place-items-center p-8" role="status"><div class="space-y-3 text-center"><div class="mx-auto size-6 animate-spin rounded-full border-2 border-surface-300-700 border-t-primary-500"></div><p class="text-sm text-surface-500">Loading {firewallSelectedBackend} status…</p></div></div>{:else if firewallContentError}<div class="system-table-body p-4"><p class="rounded-base border border-error-500/40 bg-error-500/10 p-3 text-sm" role="alert">{firewallContentError}</p></div>{:else}
            <div class="flex flex-wrap items-center gap-3 border-b border-surface-200-800 px-3 py-2 text-sm"><span class="font-medium uppercase">{firewall.backend}</span><span class="w-fit rounded-full border px-2 py-0.5 text-xs {firewall.unknown ? 'border-warning-500/40 bg-warning-500/10 text-warning-700-300' : firewall.active ? 'border-success-500/40 bg-success-500/10 text-success-700-300' : 'border-error-500/40 bg-error-500/10 text-error-700-300'}">{firewall.unknown ? 'Unknown' : firewall.active ? 'Active' : 'Inactive'}</span>{#if firewall.policy}<span class="text-xs text-surface-500">{firewall.policy}</span>{/if}<div class="ml-auto flex gap-2">{#if firewall.backend === 'ufw'}<button type="button" class="btn preset-tonal" on:click={() => openFirewallDialog('policy')}>Default policy</button>{:else}<button type="button" class="btn preset-tonal" disabled={!firewall.active} title={firewall.active ? 'Change the default firewalld zone' : 'Enable firewalld before changing zones'} on:click={() => openFirewallDialog('zone')}>Default zone</button>{/if}{#if firewall.backends.length > 1}<button type="button" class="btn preset-tonal" on:click={confirmFirewallSelection}>Use exclusively</button>{/if}<button type="button" class="btn {firewall.active ? 'border-error-500/50 text-error-700-300' : 'preset-filled-primary-500'}" on:click={() => confirmFirewallToggle(firewall)}>{firewall.active ? 'Disable' : 'Enable'}</button></div></div>
            <div class="firewall-grid package-table-header grid border-b border-surface-300-700 px-3 py-2 text-xs font-semibold"><span>Rule</span><span>Action</span><span>Source</span><span>Actions</span></div>
            <div class="system-table-body">{#each filteredFirewallRules as rule}<div class="firewall-grid grid items-center border-b border-surface-200-800 px-3 py-2 text-sm"><span class="truncate font-mono text-xs" title={rule.target}>{rule.kind === 'port' ? `${rule.target}/${rule.protocol}` : rule.target}</span><span class="capitalize">{rule.action}{rule.direction ? ` ${rule.direction}` : ''}</span><span class="truncate">{rule.source}</span><span class="flex gap-1"><button type="button" class="btn preset-tonal" disabled={!rule.mutable || !firewall.active} title={!rule.mutable ? 'Rich rules are read-only in this editor' : !firewall.active ? 'Enable this backend before editing rules' : 'Edit rule'} on:click={() => openFirewallDialog('edit', rule)}>Edit</button><button type="button" class="btn border-error-500/50 text-error-700-300" disabled={!rule.mutable || !firewall.active || (rule.kind === 'service' && rule.target === 'ssh') || (rule.kind === 'port' && rule.target === '22' && rule.protocol === 'tcp')} title={!rule.mutable ? 'Rich rules are read-only in this editor' : !firewall.active ? 'Enable this backend before removing rules' : (rule.kind === 'service' && rule.target === 'ssh') || (rule.kind === 'port' && rule.target === '22' && rule.protocol === 'tcp') ? 'SSH access rules are protected from removal' : 'Remove rule'} on:click={() => confirmFirewallRuleRemoval(rule)}>Remove</button></span></div>{:else}<p class="p-5 text-sm text-surface-500" role="status">No matching firewall rules.</p>{/each}</div>
            {/if}
          {/if}
        </section>
      {:else if activeView === 'Samba Server'}
        <section class="system-table rounded-base border border-surface-300-700 bg-surface-100-800" aria-labelledby="samba-heading">
          <header class="flex items-center gap-3 border-b border-surface-300-700 p-3"><div><h2 id="samba-heading" class="font-semibold">Samba server</h2><p class="text-xs text-surface-500">Shares configured in /etc/samba/smb.conf.</p></div><div class="ml-auto flex gap-2"><button type="button" class="btn preset-tonal" on:click={openSambaGlobalDialog} disabled={!sambaServer?.configured}>Global configuration</button><button type="button" class="btn preset-filled-primary-500" on:click={() => openConfigDialog('samba')}>Add share</button><button type="button" class="btn preset-tonal" on:click={() => loadSystemView()} disabled={systemLoading}>Reload</button></div></header>
          <div class="flex gap-4 border-b border-surface-300-700 px-3 py-2 text-sm" aria-label="Samba runit services">{#each ['smbd', 'nmbd'] as name}{@const service = sambaServer?.services.find((item) => item.name === name)}<div class="flex items-center gap-2"><span class="font-mono font-medium">{name}</span><span class="w-fit rounded-full border px-2 py-0.5 text-xs {service?.status === 'Running' ? 'border-success-500/40 bg-success-500/10 text-success-700-300' : 'border-surface-400/50 bg-surface-200-800 text-surface-700-300'}">{service?.status ?? 'Not installed'}</span><span class="text-xs text-surface-500">{service ? service.enabled ? 'Enabled' : 'Disabled' : 'Unavailable'}</span></div>{/each}</div>
          {#if !sambaServer?.configured}<p class="system-table-body p-5 text-sm text-surface-500" role="status">Samba server configuration was not found.</p>{:else}<div class="system-table-body"><div class="samba-grid package-table-header sticky top-0 z-10 grid border-b border-surface-300-700 px-3 py-2 text-xs font-semibold"><span>Share</span><span>Path</span><span>Access</span><span>Guest</span><span>Actions</span></div>{#each filteredSambaShares as share}<div class="samba-grid grid items-center border-b border-surface-200-800 px-3 py-2 text-sm"><span class="truncate font-medium">{share.name}</span><span class="truncate font-mono text-xs">{share.path || '—'}</span><span>{share.readOnly ? 'Read only' : 'Writable'}</span><span>{share.guestOK ? 'Allowed' : 'Disabled'}</span><span class="flex gap-1"><button type="button" class="btn preset-tonal" on:click={() => openConfigDialog('samba', 'edit', share)}>Edit</button><button type="button" class="btn border-error-500/50 text-error-700-300" on:click={() => confirmSambaRemoval(share)}>Remove</button></span></div>{:else}<p class="p-5 text-sm text-surface-500" role="status">No matching Samba shares.</p>{/each}</div>{/if}
        </section>
      {:else if activeView === 'NFS Client'}
        <section class="system-table rounded-base border border-surface-300-700 bg-surface-100-800" aria-labelledby="nfs-heading">
          <header class="flex items-center gap-3 border-b border-surface-300-700 p-3"><div><h2 id="nfs-heading" class="font-semibold">NFS client</h2><p class="text-xs text-surface-500">Configured and mounted NFS filesystems.</p></div><div class="ml-auto flex gap-2"><button type="button" class="btn preset-filled-primary-500" on:click={() => openConfigDialog('nfs')}>Add mount</button><button type="button" class="btn preset-tonal" on:click={() => loadSystemView()} disabled={systemLoading}>Reload</button></div></header>
          <div class="system-table-body"><div class="nfs-grid package-table-header sticky top-0 z-10 grid border-b border-surface-300-700 px-3 py-2 text-xs font-semibold"><span>Source</span><span>Mount point</span><span>Type</span><span>Options</span><span>Status</span><span>Actions</span></div>{#each filteredNFSMounts as mount}<div class="nfs-grid grid items-center border-b border-surface-200-800 px-3 py-2 text-sm"><span class="truncate font-mono text-xs">{mount.source}</span><span class="truncate font-mono text-xs">{mount.target}</span><span>{mount.type}</span><span class="nfs-options truncate text-xs" title={mount.options}>{mount.options}</span><span class="w-fit rounded-full border px-2 py-0.5 text-xs {mount.mounted ? 'border-success-500/40 bg-success-500/10 text-success-700-300' : 'border-surface-400/50 bg-surface-200-800 text-surface-700-300'}">{mount.mounted ? 'Mounted' : 'Unmounted'}</span><span class="flex gap-1"><button type="button" class="btn preset-tonal" disabled={!mount.mounted && !mount.configured} title={!mount.mounted && !mount.configured ? 'Only configured NFS filesystems can be mounted' : ''} on:click={() => confirmNFSAction(mount.mounted ? 'unmount' : 'mount', mount)}>{mount.mounted ? 'Unmount' : 'Mount'}</button>{#if mount.configured}<button type="button" class="btn preset-tonal" disabled={mount.mounted} title={mount.mounted ? 'Unmount before editing persistent configuration' : ''} on:click={() => openConfigDialog('nfs', 'edit', mount)}>Edit</button><button type="button" class="btn border-error-500/50 text-error-700-300" disabled={mount.mounted} title={mount.mounted ? 'Unmount before removing persistent configuration' : ''} on:click={() => confirmNFSAction('remove', mount)}>Remove</button>{/if}</span></div>{:else}<p class="p-5 text-sm text-surface-500" role="status">No NFS mounts found.</p>{/each}</div>
        </section>
      {:else if activeView === 'Kernels'}
        <section class="system-table rounded-base border border-surface-300-700 bg-surface-100-800" aria-labelledby="kernels-heading">
          <header class="flex flex-wrap items-center gap-3 border-b border-surface-300-700 p-3"><div><h2 id="kernels-heading" class="font-semibold">Void kernels</h2><p class="text-xs text-surface-500">Install available kernels or remove kernels that pass the running-kernel and fallback safety checks.</p></div><div class="ml-auto flex gap-2"><button type="button" class="btn bg-error-600 text-white hover:bg-error-700" disabled={!purgeableKernelImages.length && !kernels.some((kernel) => kernel.canRemove)} title={purgeableKernelImages.length || kernels.some((kernel) => kernel.canRemove) ? 'Remove safely eligible old kernels' : 'No safely removable kernels'} on:click={openKernelPurge}>Purge old kernels</button><button type="button" class="btn preset-tonal" on:click={() => loadSystemView()} disabled={systemLoading}>Reload</button></div></header>
          {#if kernelStatus}<div class="grid grid-cols-6 gap-px border-b border-surface-300-700 bg-surface-300-700 text-xs max-[1050px]:grid-cols-3 max-[620px]:grid-cols-2"><div class="bg-surface-50-900 px-3 py-2"><span class="block text-surface-500">Boot mode</span><strong>{kernelStatus.bootMode}</strong></div><div class="bg-surface-50-900 px-3 py-2"><span class="block text-surface-500">Bootloader</span><strong>{kernelStatus.bootloader}</strong></div><div class="bg-surface-50-900 px-3 py-2"><span class="block text-surface-500">/boot available</span><strong>{formatBytes(kernelStatus.boot.available)}</strong></div><div class="bg-surface-50-900 px-3 py-2"><span class="block text-surface-500">/boot filesystem</span><strong>{kernelStatus.boot.fileSystem || 'Unknown'}{kernelStatus.boot.separate ? ' · separate' : ''}</strong></div><div class="bg-surface-50-900 px-3 py-2"><span class="block text-surface-500">EFI</span><strong>{kernelStatus.bootMode === 'UEFI' ? kernelStatus.efi.mounted ? `${kernelStatus.efi.fileSystem} mounted` : 'Not mounted separately' : 'Not used'}</strong></div><div class="bg-surface-50-900 px-3 py-2"><span class="block text-surface-500">Initramfs</span><strong>{kernelStatus.initramfsTool}</strong></div></div>{#each kernelStatus.warnings as warning}<p class="border-b border-warning-500/30 bg-warning-500/10 px-3 py-1.5 text-xs" role="status">{warning}</p>{/each}{/if}
          <div class="flex gap-1 border-b border-surface-300-700 bg-surface-50-900 px-3 pt-2" role="tablist" aria-label="Kernel information"><button type="button" role="tab" aria-selected={kernelTab === 'packages'} class="border-b-2 px-3 py-2 text-sm {kernelTab === 'packages' ? 'border-primary-500 font-semibold text-primary-700-300' : 'border-transparent text-surface-500'}" on:click={() => kernelTab = 'packages'}>Packages</button><button type="button" role="tab" aria-selected={kernelTab === 'images'} class="border-b-2 px-3 py-2 text-sm {kernelTab === 'images' ? 'border-primary-500 font-semibold text-primary-700-300' : 'border-transparent text-surface-500'}" on:click={() => kernelTab = 'images'}>Boot images <span class="text-xs">{kernelStatus?.images.length ?? 0}</span></button><button type="button" role="tab" aria-selected={kernelTab === 'dkms'} class="border-b-2 px-3 py-2 text-sm {kernelTab === 'dkms' ? 'border-primary-500 font-semibold text-primary-700-300' : 'border-transparent text-surface-500'}" on:click={() => kernelTab = 'dkms'}>DKMS <span class="text-xs">{kernelStatus?.dkms.length ?? 0}</span></button></div>
          {#if kernelTab === 'packages'}<div class="kernel-grid package-table-header grid border-b border-surface-300-700 px-3 py-2 text-xs font-semibold"><span>Kernel</span><span>Version</span><span class="kernel-type">Type</span><span>Size</span><span class="kernel-date">Installed date</span><span>Status</span><span>Actions</span></div>
          <div class="system-table-body">{#each filteredKernels as item}<div class="kernel-grid grid items-center border-b border-surface-200-800 px-3 py-2 text-sm hover:bg-primary-500/5"><span class="truncate font-medium" title={item.package}>{item.package}</span><span class="truncate font-mono text-xs" title={item.version}>{item.version || '—'}</span><span class="kernel-type">{item.type || 'Stable'}</span><span class="font-mono text-xs">{item.size || '—'}</span><span class="kernel-date truncate text-xs" title={item.installDate}>{item.installDate || '—'}</span><span class="w-fit rounded-full border px-2 py-0.5 text-xs {item.running ? 'border-primary-500/40 bg-primary-500/10 text-primary-700-300' : item.meta ? 'border-warning-500/40 bg-warning-500/10 text-warning-700-300' : item.installed ? 'border-success-500/40 bg-success-500/10 text-success-700-300' : 'border-surface-400/50 bg-surface-200-800 text-surface-700-300'}">{item.running ? 'Running' : item.meta ? 'Meta' : item.installed ? 'Installed' : 'Available'}</span><span class="flex flex-wrap gap-1">{#if item.canInstall}<button type="button" class="btn preset-filled-primary-500" on:click={() => openNamedPackageTransaction('Install kernel', `Install ${item.package}.`, [item.package], 'install')}>Install</button>{:else if item.installed && !item.meta}<span class="inline-flex gap-1 whitespace-nowrap"><button type="button" class="btn preset-tonal" disabled={systemLoading} title="Re-run post-install hooks and regenerate initramfs" on:click={() => reconfigureKernel(item)}>Reconfigure</button>{#if !item.running}<button type="button" class="btn preset-tonal" disabled={!kernelStatus?.switchSupported} title={kernelStatus?.switchSupported ? 'Use after the next restart' : kernelStatus?.switchReason} on:click={() => confirmDefaultKernel(item)}>Set default</button>{/if}</span>{#if item.canRemove}<button type="button" class="btn border-error-500/50 text-error-700-300 hover:bg-error-500/10" on:click={() => openNamedPackageTransaction('Remove kernel', `Remove ${item.package}. The running kernel and one fallback remain protected.`, [item.package], 'remove')}>Remove</button>{/if}{/if}</span></div>{/each}</div>
          {:else if kernelTab === 'images'}<div class="kernel-image-grid package-table-header grid border-b border-surface-300-700 px-3 py-2 text-xs font-semibold"><span>Boot image</span><span>Size</span><span>Status</span><span>Removal</span></div><div class="system-table-body">{#each kernelStatus?.images ?? [] as image}<div class="kernel-image-grid grid items-center border-b border-surface-200-800 px-3 py-2 text-sm"><span class="truncate font-mono">vmlinuz-{image.version}</span><span class="font-mono text-xs">{formatBytes(image.size)}</span><span class="w-fit rounded-full border px-2 py-0.5 text-xs {image.status === 'Running' ? 'border-primary-500/40 bg-primary-500/10 text-primary-700-300' : image.status === 'Old' ? 'border-warning-500/40 bg-warning-500/10 text-warning-700-300' : image.status === 'Managed' ? 'border-success-500/40 bg-success-500/10 text-success-700-300' : 'border-error-500/40 bg-error-500/10 text-error-700-300'}">{image.status}</span><span>{#if image.canPurge}<button type="button" class="btn border-error-500/50 text-error-700-300" on:click={() => openMaintenance('Purge old kernel', `Remove obsolete kernel ${image.version} with vkpurge.`, 'purge-kernels', true, [image.version])}>Purge</button>{:else}<span class="text-xs text-surface-500">{image.status === 'Manual' ? 'Manual review required' : 'Protected'}</span>{/if}</span></div>{:else}<p class="p-5 text-sm text-surface-500">No kernel boot images found.</p>{/each}</div>
          {:else}<div class="kernel-dkms-grid package-table-header grid border-b border-surface-300-700 px-3 py-2 text-xs font-semibold"><span>Module</span><span>Version</span><span>Kernel</span><span>Architecture</span><span>Status</span><span>Actions</span></div><div class="system-table-body">{#each kernelStatus?.dkms ?? [] as module}<div class="kernel-dkms-grid grid items-center border-b border-surface-200-800 px-3 py-2 text-sm"><span class="truncate font-medium">{module.module}</span><span class="truncate font-mono text-xs">{module.version}</span><span class="truncate font-mono text-xs">{module.kernel || '—'}</span><span>{module.arch}</span><span class="w-fit rounded-full border px-2 py-0.5 text-xs {module.status === 'installed' || module.installed ? 'border-success-500/40 bg-success-500/10 text-success-700-300' : 'border-warning-500/40 bg-warning-500/10 text-warning-700-300'}">{module.status}</span><span>{#if module.package && !module.installed}<button type="button" class="btn preset-filled-primary-500" on:click={() => openNamedPackageTransaction('Install DKMS module', `Install ${module.module}.`, [module.module], 'install')}>Install</button>{:else if module.package && module.installed}<button type="button" class="btn border-error-500/50 text-error-700-300 hover:bg-error-500/10" on:click={() => openNamedPackageTransaction('Remove DKMS module', `Remove ${module.module}.`, [module.module], 'remove')}>Remove</button>{/if}</span></div>{:else}<p class="p-5 text-sm text-surface-500">No DKMS modules available.</p>{/each}</div>{/if}
        </section>
      {:else if activeView === 'Repositories'}
        <div class="grid min-h-0 gap-3 overflow-auto"><section class="system-table rounded-base border border-surface-300-700 bg-surface-100-800" aria-labelledby="repositories-heading">
          <header class="flex items-center gap-3 border-b border-surface-300-700 p-3"><div><h2 id="repositories-heading" class="font-semibold">Effective XBPS repositories</h2><p class="text-xs text-surface-500">Listed in XBPS priority order.</p></div><div class="ml-auto flex gap-2"><button class="btn preset-tonal" type="button" on:click={() => openConfigDialog('repository')}>Add repository</button><button type="button" class="btn preset-tonal" on:click={() => loadSystemView()} disabled={systemLoading}>Reload</button></div></header>
          <div class="system-table-body">{#each filteredRepositories as repository}
            <div class="repository-grid grid items-center gap-2 border-b border-surface-200-800 px-3 py-2 text-sm">
              <button type="button" role="switch" aria-checked={repository.enabled} aria-label={`${repository.enabled ? 'Disable' : 'Enable'} ${repository.url}`} disabled={!repository.mutable} title={repository.mutable ? '' : 'Only Sysentra-owned repositories can be changed'} class="relative h-5 w-9 rounded-full disabled:opacity-50 {repository.enabled ? 'bg-success-600' : 'bg-surface-400'}" on:click={() => confirmRepositoryToggle(repository)}><span class="absolute top-0.5 size-4 rounded-full bg-white transition-[left] {repository.enabled ? 'left-[1.125rem]' : 'left-0.5'}"></span></button>
              <span class="repository-priority">#{repository.priority}</span><span class="truncate font-mono text-xs">{repository.url}</span>
              <button type="button" class="btn preset-tonal" disabled={!repository.mutable} title={repository.mutable ? 'Edit this Sysentra-owned repository' : 'Only app-owned repository files may be edited'} on:click={() => openConfigDialog('repository', 'edit', repository)}>Edit</button>
              <button type="button" class="btn border-error-500/50 text-error-700-300 hover:bg-error-500/10" disabled={!repository.mutable} title={repository.mutable ? 'Remove this configured repository' : 'Void vendor repositories cannot be removed here'} on:click={() => confirmRepositoryRemoval(repository)}>Remove</button>
            </div>
          {/each}</div>
          <footer class="p-3 text-xs text-surface-500">Only Sysentra-managed entries can be changed.</footer>
        </section>
        {#if flatpakSourceEnabled}<section class="system-table rounded-base border border-surface-300-700 bg-surface-100-800" aria-labelledby="flatpak-remotes-heading">
          <header class="flex items-center gap-3 border-b border-surface-300-700 p-3"><div><h2 id="flatpak-remotes-heading" class="font-semibold">User Flatpak remotes</h2><p class="text-xs text-surface-500">Package sources for the current user.</p></div><button class="btn preset-tonal ml-auto" type="button" on:click={() => openConfigDialog('flatpak')}>Add remote</button></header>
          <div class="package-table-header grid grid-cols-[3rem_minmax(7rem,1fr)_minmax(10rem,3fr)_auto] gap-2 border-b border-surface-300-700 px-3 py-2 text-xs font-semibold"><span>Active</span><span>Name</span><span>URL</span><span>Actions</span></div>
          <div class="system-table-body">{#each filteredFlatpakRemotes as remote}<div class="grid grid-cols-[3rem_minmax(7rem,1fr)_minmax(10rem,3fr)_auto] items-center gap-2 border-b border-surface-200-800 px-3 py-2 text-sm"><button type="button" role="switch" aria-checked={remote.enabled} aria-label={`${remote.enabled ? 'Disable' : 'Enable'} ${remote.name}`} class="relative h-5 w-9 rounded-full {remote.enabled ? 'bg-success-600' : 'bg-surface-400'}" on:click={() => confirmFlatpakRemoteToggle(remote)}><span class="absolute top-0.5 size-4 rounded-full bg-white transition-[left] {remote.enabled ? 'left-[1.125rem]' : 'left-0.5'}"></span></button><span class="truncate font-medium" title={remote.title || remote.name}>{remote.title || remote.name}</span><span class="truncate font-mono text-xs" title={remote.url}>{remote.url}</span><span class="flex gap-1"><button type="button" class="btn preset-tonal" on:click={() => openConfigDialog('flatpak', 'edit', remote)}>Edit</button><button type="button" class="btn border-error-500/50 text-error-700-300" on:click={() => confirmFlatpakRemoteRemoval(remote)}>Remove</button></span></div>{:else}<p class="p-5 text-sm text-surface-500">No user Flatpak remotes configured.</p>{/each}</div>
        </section>{/if}</div>
      {:else if activeView === 'XBPS Config'}
        <section class="system-table rounded-base border border-surface-300-700 bg-surface-100-800" aria-labelledby="xbps-config-heading">
          <header class="flex flex-wrap items-center gap-3 border-b border-surface-300-700 p-3"><div><h2 id="xbps-config-heading" class="font-semibold">Advanced XBPS configuration</h2><p class="text-xs text-surface-500">Ignored packages and virtual package mappings from the effective XBPS configuration.</p></div><div class="ml-auto flex gap-2"><button type="button" class="btn preset-filled-primary-500" on:click={() => openRuleDialog()}>Add rule</button><button type="button" class="btn preset-tonal" on:click={() => loadSystemView()} disabled={systemLoading}>Reload</button></div></header>
          <div class="xbps-config-grid package-table-header grid border-b border-surface-300-700 px-3 py-2 text-xs font-semibold"><span>Type</span><span>Value</span><span class="xbps-config-origin">Configuration file</span><span>Actions</span></div>
          <div class="system-table-body">{#each filteredXBPSConfigRules as rule}
            <div class="xbps-config-grid grid items-center border-b border-surface-200-800 px-3 py-2 text-sm"><span class="w-fit rounded-full border border-primary-500/40 bg-primary-500/10 px-2 py-0.5 text-xs text-primary-700-300">{rule.kind}</span><span class="truncate font-mono text-xs" title={rule.value}>{rule.value}</span><span class="xbps-config-origin truncate font-mono text-xs text-surface-500" title={rule.origin}>{rule.origin}</span><span class="flex gap-1"><button type="button" class="btn preset-tonal" disabled={!rule.mutable} title={rule.mutable ? 'Edit this Sysentra-owned rule' : 'Only Sysentra-owned rules may be edited'} on:click={() => openRuleDialog('edit', rule)}>Edit</button><button type="button" class="btn border-error-500/50 text-error-700-300" disabled={!rule.mutable} title={rule.mutable ? 'Remove this Sysentra-owned rule' : 'Vendor and user-owned files are read-only'} on:click={() => confirmRuleRemoval(rule)}>Remove</button></span></div>
          {:else}<p class="p-5 text-sm text-surface-500" role="status">No ignored packages or virtual package mappings found.</p>{/each}</div>
          <footer class="p-3 text-xs text-surface-500">Ignored packages satisfy dependencies without installation. Virtual mappings replace a virtual name with a package.</footer>
        </section>
      {:else if activeView === 'Mirrors'}
        <section class="mirror-table system-table rounded-base border border-surface-300-700 bg-surface-100-800" aria-labelledby="mirrors-heading">
          <header class="flex flex-wrap items-center gap-3 border-b border-surface-300-700 p-3">
            <div><h2 id="mirrors-heading" class="font-semibold">Void repository mirrors</h2><p class="text-xs text-surface-500">Official xmirror catalog with live repository sync age.</p></div>
            <div class="ml-auto flex gap-2"><button type="button" class="btn preset-tonal" on:click={() => confirmMirror('')}>Reset to default</button><button type="button" class="btn preset-tonal" on:click={() => loadSystemView()} disabled={systemLoading}>Reload</button></div>
          </header>
          {#each mirrorWarnings as warning}<p class="border-b border-warning-500/30 bg-warning-500/10 px-3 py-2 text-sm" role="status">{warning}</p>{/each}
          {#if mirrorRecoveryError}<p class="border-b border-error-500/30 bg-error-500/10 px-3 py-2 text-sm" role="alert">{mirrorRecoveryError}</p>{/if}
          <div class="flex flex-wrap items-center gap-2 border-b border-surface-300-700 p-2" aria-label="Mirror filters">
            {#each [[0, 'All tiers'], [1, 'Tier 1'], [2, 'Tier 2']] as filter}<button type="button" class="rounded-base px-2.5 py-1 text-xs {mirrorTier === filter[0] ? 'bg-primary-500/15 font-semibold text-primary-700-300' : 'hover:bg-surface-200-800'}" aria-pressed={mirrorTier === filter[0]} on:click={() => mirrorTier = Number(filter[0])}>{filter[1]}</button>{/each}
            <div class="ml-auto flex flex-wrap items-center justify-end gap-3"><label class="flex items-center gap-2 text-xs"><span>Region</span><select class="select app-input-surface h-8 py-1" bind:value={mirrorRegion}>{#each mirrorRegions as region}<option>{region}</option>{/each}</select></label><button type="button" role="switch" aria-checked={mirrorRecoveryEnabled} class="flex items-center gap-2 text-xs" disabled={mirrorRecoverySaving} on:click={toggleMirrorRecovery}><span class="relative h-5 w-9 rounded-full {mirrorRecoveryEnabled ? 'bg-success-600' : 'bg-surface-400'}"><span class="absolute top-0.5 size-4 rounded-full bg-white transition-[left] {mirrorRecoveryEnabled ? 'left-[1.125rem]' : 'left-0.5'}"></span></span><span>{mirrorRecoverySaving ? 'Saving…' : 'Recover from failed mirror'}</span></button></div>
          </div>
          <div class="mirror-grid package-table-header grid border-b border-surface-300-700 px-3 py-2 text-left text-xs font-semibold"><span>Mirror</span><span class="mirror-region">Region</span><span class="mirror-location">Location</span><span class="mirror-tier">Tier</span><span class="mirror-synced">Last synced</span><span>Status</span><span>Action</span></div>
          <div class="system-table-body">{#each filteredMirrors as mirror}
            <div class="mirror-grid grid border-b border-surface-200-800 px-3 py-2 text-left text-sm hover:bg-primary-500/5">
              <a class="truncate font-mono text-xs text-primary-700-300 underline hover:no-underline" href={mirror.url} title={mirror.url} on:click|preventDefault={() => openExternalURL(mirror.url)}>{mirror.url}</a><span class="mirror-region truncate">{mirror.region}</span><span class="mirror-location truncate" title={mirror.location}>{mirror.location}</span><span class="mirror-tier">Tier {mirror.tier}</span><span class="mirror-synced truncate text-xs" title={mirror.lastSynced}>{mirror.lastSynced}</span>
              <span class="mirror-status rounded-full border px-2 py-0.5 text-center text-xs {mirror.status === 'Healthy' ? 'border-success-500/40 bg-success-500/10 text-success-700-300' : mirror.status === 'Delayed' ? 'border-warning-500/40 bg-warning-500/10 text-warning-700-300' : 'border-error-500/40 bg-error-500/10 text-error-700-300'}">{mirror.status}</span>
              <button type="button" class="btn mirror-action {mirror.current ? 'preset-filled-primary-500' : 'preset-tonal'}" disabled={mirror.current || mirror.status === 'Unreachable'} title={mirror.current ? 'Currently selected' : mirror.status === 'Unreachable' ? 'Unavailable mirrors cannot be selected' : 'Use this mirror'} on:click={() => confirmMirror(mirror.url)}>{mirror.current ? 'Current' : 'Use mirror'}</button>
            </div>
          {:else}<p class="p-5 text-sm text-surface-500" role="status">No mirrors match these filters.</p>{/each}</div>
          <footer class="p-3 text-xs text-surface-500">Void maintains Tier 1 mirrors. Tier 2 mirrors provide no freshness guarantee.</footer>
        </section>
      {:else if activeView === 'Firmware'}
        <section class="system-table rounded-base border border-surface-300-700 bg-surface-100-800" aria-labelledby="firmware-heading">
          <header class="flex items-center gap-3 border-b border-surface-300-700 p-3"><div><h2 id="firmware-heading" class="font-semibold">Firmware devices</h2><p class="text-xs text-surface-500">Device and reviewed release information reported by fwupd.</p></div><div class="ml-auto flex gap-2"><button type="button" class="btn preset-filled-primary-500" disabled={!firmwareAvailable || !!firmwareWarning || systemLoading} on:click={() => openFirmwareAction('refresh')}>Refresh metadata</button><button type="button" class="btn preset-tonal" on:click={() => loadSystemView()} disabled={systemLoading}>Reload</button></div></header>
          {#if !firmwareAvailable}
            <div class="space-y-3 p-5"><p class="text-sm text-surface-600-400" role="status">fwupd is not installed, so firmware devices and updates are unavailable.</p><button type="button" class="btn preset-tonal" on:click={() => openNamedPackageTransaction('Install fwupd', 'Install fwupd firmware update support.', ['fwupd'], 'install')}>Install fwupd</button></div>
          {:else if firmwareWarning}
            <p class="p-5 text-sm text-warning-700-300" role="status">{firmwareWarning}</p>
          {:else if firmwareDevices.length === 0}
            <p class="p-5 text-sm text-surface-600-400" role="status">fwupd did not report any devices.</p>
          {:else}
            <div class="system-table-body">
              <div class="firmware-grid package-table-header sticky top-0 z-10 grid border-b border-surface-300-700 px-3 py-2 text-xs font-semibold">
                <span>Device</span><span class="firmware-vendor">Vendor</span><span>Current</span><span class="firmware-available">Available</span><span>Status</span><span>Action</span>
              </div>
              {#each filteredFirmwareDevices as device}
                <div class="firmware-grid grid items-center border-b border-surface-200-800 px-3 py-2 text-sm">
                  <div class="min-w-0"><p class="truncate font-medium">{device.name || 'Unnamed device'}</p><p class="truncate text-xs text-surface-500">{device.summary}</p></div>
                  <span class="firmware-vendor truncate">{device.vendor || '—'}</span><span class="truncate font-mono text-xs">{device.version || '—'}</span><span class="firmware-available truncate font-mono text-xs">{device.newVersion || '—'}</span>
                  <span class="w-fit rounded-full border px-2 py-0.5 text-xs {device.newVersion ? 'border-warning-500/40 bg-warning-500/10 text-warning-700-300' : device.updatable ? 'border-success-500/40 bg-success-500/10 text-success-700-300' : 'border-surface-400/50 bg-surface-200-800 text-surface-700-300'}">{device.newVersion ? device.shutdownRequired ? 'Update · shutdown' : device.rebootRequired ? 'Update · reboot' : 'Update available' : device.updatable ? 'Up to date' : 'Not updatable'}</span><button type="button" class="btn preset-tonal" disabled={!device.newVersion} title={device.newVersion ? `Review update ${device.newVersion}` : 'No update is available'} on:click={() => openFirmwareAction('update', device)}>Review</button>
                </div>
              {/each}
            </div>
          {/if}
        </section>
      {:else if activeView === 'Drivers'}
        <section class="system-table rounded-base border border-surface-300-700 bg-surface-100-800" aria-labelledby="drivers-heading">
          <header class="flex items-center gap-3 border-b border-surface-300-700 p-3"><div><h2 id="drivers-heading" class="font-semibold">Hardware drivers</h2><p class="text-xs text-surface-500">PCI and USB devices with their currently bound kernel drivers.</p></div><button type="button" class="btn preset-tonal ml-auto" on:click={() => loadSystemView()} disabled={systemLoading}>Reload</button></header>
          {#each driverWarnings as warning}<p class="border-b border-warning-500/30 bg-warning-500/10 px-3 py-2 text-sm" role="status">{warning}</p>{/each}
          <div class="flex flex-wrap gap-1 border-b border-surface-300-700 p-2" aria-label="Driver category filters">{#each driverCategories as category}<button type="button" class="rounded-base px-2.5 py-1 text-xs {driverCategory === category ? 'bg-primary-500/15 font-semibold text-primary-700-300' : 'hover:bg-surface-200-800'}" aria-pressed={driverCategory === category} on:click={() => driverCategory = category}>{category}</button>{/each}</div>
          <div class="driver-grid package-table-header grid border-b border-surface-300-700 px-3 py-2 text-xs font-semibold"><span>Device</span><span class="driver-bus">Bus / ID</span><span>Kernel driver</span><span>Status</span></div>
          <div class="system-table-body">{#each filteredDriverDevices as device}<div class="driver-grid grid items-center border-b border-surface-200-800 px-3 py-2 text-sm"><div class="min-w-0"><p class="truncate font-medium">{device.name}</p><p class="truncate text-xs text-surface-500">{device.category} · {device.evidence}</p></div><span class="driver-bus truncate font-mono text-xs">{device.bus} {device.id}</span><span class="truncate font-mono text-xs">{device.driver || '—'}</span><span class="w-fit rounded-full border px-2 py-0.5 text-xs {device.driver ? 'border-success-500/40 bg-success-500/10 text-success-700-300' : 'border-warning-500/40 bg-warning-500/10 text-warning-700-300'}">{device.status}</span></div>{:else}<p class="p-5 text-sm text-surface-500" role="status">No matching devices.</p>{/each}</div>
        </section>
      {/if}
    </div>
  </main>

  <footer class="app-chrome col-span-2 flex items-center gap-4 border-t border-surface-300-700 bg-surface-100-800 px-3 text-[0.6875rem] text-surface-500 max-[760px]:col-span-1">
    <span>{activeView === 'Services' ? 'runit' : activeView === 'Users & Groups' ? 'Local accounts' : activeView === 'Firewall' ? firewall?.backend || 'Firewall' : activeView === 'Samba Server' ? 'Samba' : activeView === 'NFS Client' ? 'NFS' : activeView === 'Kernels' || activeView === 'Repositories' || activeView === 'XBPS Config' ? 'XBPS' : activeView === 'Mirrors' ? 'xmirror' : activeView === 'Firmware' ? 'fwupd' : activeView === 'Drivers' ? 'PCI + USB' : flatpakSourceEnabled ? 'XBPS + Flatpak' : 'XBPS'}</span>
    <span>{activeView === 'Services' ? `${filteredServices.length} services` : activeView === 'Users & Groups' ? `${accountTab === 'users' ? filteredUsers.length + ' users' : filteredGroups.length + ' groups'}` : activeView === 'Firewall' ? `${filteredFirewallRules.length} rules` : activeView === 'Samba Server' ? `${filteredSambaShares.length} shares` : activeView === 'NFS Client' ? `${filteredNFSMounts.length} mounts` : activeView === 'Kernels' ? `${filteredKernels.length} kernels` : activeView === 'Repositories' ? `${filteredRepositories.length} repositories` : activeView === 'XBPS Config' ? `${filteredXBPSConfigRules.length} rules` : activeView === 'Mirrors' ? `${filteredMirrors.length} mirrors` : activeView === 'Firmware' ? `${filteredFirmwareDevices.length} devices` : activeView === 'Drivers' ? `${filteredDriverDevices.length} devices` : `${packageResultCount.toLocaleString()} ${activeView === 'Discover' ? 'available' : activeView === 'Installed' ? 'installed' : activeView === 'Updates' ? 'updates' : ''}`}</span>
    <span class="ml-auto">{systemName}</span>
  </footer>

  <dialog bind:this={settingsDialog} class="m-auto w-[min(34rem,calc(100%-2rem))] rounded-base border border-surface-300-700 bg-surface-100-800 p-0 text-surface-950-50 shadow-xl backdrop:bg-black/45" aria-labelledby="settings-title">
    <div>
      <header class="border-b border-surface-300-700 px-4 py-3"><h2 id="settings-title" class="font-semibold">Settings</h2><p class="text-xs text-surface-500">Appearance and optional package sources.</p></header>
      <div class="space-y-5 p-4">
        <section aria-labelledby="appearance-heading" class="space-y-3">
          <h3 id="appearance-heading" class="text-sm font-semibold">Appearance</h3>
          <fieldset><legend class="mb-1 text-xs font-medium text-surface-600-400">Color mode</legend><div class="grid grid-cols-3 gap-1 rounded-base bg-surface-200-800 p-1">{#each ['auto', 'light', 'dark'] as mode}<button type="button" class="settings-mode-choice {colorMode === mode ? 'settings-mode-active' : ''}" aria-pressed={colorMode === mode} on:click={() => setColorMode(mode as 'auto' | 'light' | 'dark')}>{#if colorMode === mode}<svg viewBox="0 0 20 20" class="size-3.5" aria-hidden="true"><path d="m4.5 10.5 3.3 3.2 7.7-8" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>{/if}<span>{mode === 'auto' ? 'System' : mode}</span></button>{/each}</div></fieldset>
          <label class="block text-sm"><span class="mb-1 block text-xs font-medium text-surface-600-400">Theme</span><select class="select app-input-surface w-full" bind:value={theme} on:change={() => applyAppearance()}>{#each themes as option}<option value={option[0]}>{option[1]}</option>{/each}</select></label>
          <div class="grid grid-cols-[minmax(0,1fr)_10rem] items-end gap-3 max-[500px]:grid-cols-1">
            <label class="block text-sm"><span class="mb-1 block text-xs font-medium text-surface-600-400">Font</span><select class="select app-input-surface w-full" bind:value={font} on:change={() => applyAppearance()}><option value="system">System UI</option>{#each systemFonts as family}<option value={family}>{family}</option>{/each}</select></label>
            <label class="block text-sm"><span class="mb-1 block text-xs font-medium text-surface-600-400">Font size</span><span class="flex h-8 overflow-hidden rounded-base border border-surface-300-700 bg-surface-50-900"><button type="button" class="grid w-9 shrink-0 place-items-center border-r border-surface-300-700 text-base hover:bg-surface-200-800 disabled:opacity-40" aria-label="Decrease font size" disabled={fontSize <= 10} on:click={() => setFontSize(fontSize - 1)}>−</button><input class="font-size-input min-w-0 flex-1 bg-transparent px-2 text-center font-medium text-surface-950-50 tabular-nums" type="number" min="10" max="24" step="1" value={fontSize} aria-label="Font size in pixels" on:change={(event) => setFontSize(event.currentTarget.valueAsNumber)} /><button type="button" class="grid w-9 shrink-0 place-items-center border-l border-surface-300-700 text-base hover:bg-surface-200-800 disabled:opacity-40" aria-label="Increase font size" disabled={fontSize >= 24} on:click={() => setFontSize(fontSize + 1)}>+</button></span></label>
          </div>
          <p class="rounded-base border border-surface-300-700 bg-surface-50-900 p-3 text-sm">The quick brown fox jumps over the lazy dog.</p>
        </section>
        <section aria-labelledby="sources-heading" class="space-y-2 border-t border-surface-300-700 pt-4">
          <div><h3 id="sources-heading" class="text-sm font-semibold">Additional package sources</h3><p class="text-xs text-surface-500">Disabled sources are not queried or shown in package lists.</p></div>
          {#each additionalSources as source}<div class="flex items-center gap-3 rounded-base border border-surface-300-700 px-3 py-2"><div class="min-w-0 flex-1"><p class="text-sm font-medium">{source.name}</p><p class="text-xs text-surface-500">Optional {source.id} catalog</p></div><button type="button" role="switch" aria-checked={source.enabled} aria-label={`${source.enabled ? 'Disable' : 'Enable'} ${source.name}`} class="relative h-5 w-9 shrink-0 rounded-full transition-colors {source.enabled ? 'bg-success-600' : 'bg-surface-400'}" disabled={sourceSaving === source.id} on:click={() => toggleAdditionalSource(source)}><span class="absolute left-0.5 top-0.5 size-4 rounded-full bg-white shadow transition-transform {source.enabled ? 'translate-x-4' : ''}"></span></button></div>{:else}<p class="text-sm text-surface-500">No optional package sources are configured.</p>{/each}
          {#if settingsError}<p class="rounded-base border border-error-500/40 bg-error-500/10 p-3 text-xs" role="alert">{settingsError}</p>{/if}
        </section>
      </div>
      <footer class="flex justify-end border-t border-surface-300-700 px-4 py-3"><button type="button" class="btn preset-tonal" on:click={() => settingsDialog.close()}>Close</button></footer>
    </div>
  </dialog>

  <dialog bind:this={serviceDialog} class="m-auto w-[min(32rem,calc(100%-2rem))] rounded-base border border-surface-300-700 bg-surface-100-800 p-0 text-surface-950-50 shadow-xl backdrop:bg-black/45" aria-labelledby="service-dialog-title">
    <form on:submit|preventDefault={submitService}>
      <header class="border-b border-surface-300-700 px-4 py-3"><h2 id="service-dialog-title" class="font-semibold">Add {serviceScope} service</h2></header>
      <div class="space-y-3 p-4">
        <label class="block text-sm"><span class="mb-1 block font-medium">Service name</span><input class="input app-input-surface w-full" bind:value={serviceName} pattern="[A-Za-z0-9][A-Za-z0-9_.@-]*" maxlength="128" autocomplete="off" required /></label>
        <label class="block text-sm"><span class="mb-1 block font-medium">Command</span><input class="input app-input-surface w-full font-mono" bind:value={serviceCommand} placeholder="program --flag" maxlength="16384" autocomplete="off" required /></label>
        <div class="grid gap-3 sm:grid-cols-2"><label class="block text-sm"><span class="mb-1 block font-medium">User <span class="font-normal text-surface-500">(optional)</span></span><input class="input app-input-surface w-full" bind:value={serviceUser} maxlength="32" autocomplete="off" /></label><label class="block text-sm"><span class="mb-1 block font-medium">Group <span class="font-normal text-surface-500">(optional)</span></span><input class="input app-input-surface w-full" bind:value={serviceGroup} maxlength="32" autocomplete="off" /></label></div>
        <p class="text-xs text-surface-500">The service is created disabled. Review it in the table, then enable it with the switch.</p>
        {#if serviceDialogError}<p class="rounded-base border border-error-500/40 bg-error-500/10 p-3 text-xs" role="alert">{serviceDialogError}</p>{/if}
      </div>
      <footer class="flex justify-end gap-2 border-t border-surface-300-700 px-4 py-3"><button type="button" class="btn preset-tonal" disabled={serviceSaving} on:click={() => serviceDialog.close()}>Cancel</button><button type="submit" class="btn preset-filled-primary-500" disabled={serviceSaving || !serviceName.trim() || !serviceCommand.trim()}>{serviceSaving ? 'Adding…' : 'Add service'}</button></footer>
    </form>
  </dialog>

  <dialog bind:this={kernelReconfigureDialog} class="m-auto w-[min(48rem,calc(100%-2rem))] rounded-base border border-surface-300-700 bg-surface-100-800 p-0 text-surface-950-50 shadow-xl backdrop:bg-black/45" aria-labelledby="kernel-reconfigure-title">
    <div><header class="border-b border-surface-300-700 px-4 py-3"><h2 id="kernel-reconfigure-title" class="font-semibold">{kernelReconfigureTitle}</h2></header><div class="space-y-3 p-4">
      {#if kernelReconfigureLoading}<div class="space-y-2" role="status" aria-live="polite"><p class="text-sm">Regenerating kernel configuration and initramfs…</p><div class="h-1.5 overflow-hidden rounded-full bg-surface-300-700"><span class="block h-full w-1/3 animate-pulse rounded-full bg-primary-500"></span></div></div>{/if}
      <section class="overflow-hidden rounded-base border border-surface-300-700 bg-surface-950 text-surface-50" aria-label="Kernel reconfiguration logs"><pre class="max-h-[50vh] min-h-36 overflow-auto whitespace-pre-wrap break-words p-3 font-mono text-xs">{kernelReconfigureLogs.join('\n')}</pre></section>
      {#if kernelReconfigureError}<p class="rounded-base border border-error-500/40 bg-error-500/10 p-3 text-sm text-error-700-300" role="alert">Reconfiguration failed: {kernelReconfigureError}</p>{:else if kernelReconfigureSucceeded}<p class="rounded-base border border-success-500/40 bg-success-500/10 p-3 text-sm text-success-700-300" role="status">Kernel reconfiguration completed.</p>{/if}
    </div><footer class="flex justify-end border-t border-surface-300-700 px-4 py-3"><button type="button" class="btn preset-tonal" disabled={kernelReconfigureLoading} on:click={() => kernelReconfigureDialog.close()}>Close</button></footer></div>
  </dialog>

  <dialog bind:this={historyDialog} class="m-auto w-[min(48rem,calc(100%-2rem))] rounded-base border border-surface-300-700 bg-surface-100-800 p-0 text-surface-950-50 shadow-xl backdrop:bg-black/45" aria-labelledby="history-title">
    <div><header class="border-b border-surface-300-700 px-4 py-3"><h2 id="history-title" class="font-semibold">Transaction history</h2></header><div class="max-h-[60vh] overflow-auto">{#if historyError}<p class="m-4 rounded-base border border-error-500/40 bg-error-500/10 p-3 text-sm" role="alert">{historyError}</p>{:else if transactionHistory.length}<table class="w-full table-fixed text-left text-sm"><thead class="sticky top-0 bg-surface-200-800 text-xs"><tr><th class="w-[24%] px-3 py-2">Time</th><th class="w-[22%] px-3 py-2">Action</th><th class="w-[16%] px-3 py-2">Status</th><th class="px-3 py-2">Message</th></tr></thead><tbody>{#each transactionHistory as item}<tr class="border-t border-surface-300-700"><td class="truncate px-3 py-2 font-mono text-xs" title={item.time}>{new Date(item.time).toLocaleString()}</td><td class="truncate px-3 py-2">{item.operation}</td><td class="px-3 py-2"><span class="rounded-full border px-2 py-0.5 text-xs {item.status === 'completed' ? 'border-success-500/40 bg-success-500/10 text-success-700-300' : 'border-error-500/40 bg-error-500/10 text-error-700-300'}">{item.status}</span></td><td class="truncate px-3 py-2 text-xs" title={item.message}>{item.message || '—'}</td></tr>{/each}</tbody></table>{:else}<p class="p-5 text-sm text-surface-500">No package transactions recorded.</p>{/if}</div><footer class="flex justify-end border-t border-surface-300-700 px-4 py-3"><button type="button" class="btn preset-tonal" on:click={() => historyDialog.close()}>Close</button></footer></div>
  </dialog>

  <dialog bind:this={confirmationDialog} class="m-auto w-[min(54rem,calc(100%-2rem))] rounded-base border border-surface-300-700 bg-surface-100-800 p-0 text-surface-950-50 shadow-xl backdrop:bg-black/45" aria-labelledby="confirmation-title">
    <form method="dialog">
      <header class="flex items-center gap-3 border-b border-surface-300-700 px-4 py-3"><h2 id="confirmation-title" class="font-semibold">{confirmation.title}</h2>{#if transactionPlan || transactionLogs.length}<button type="button" class="btn preset-tonal ml-auto" aria-expanded={transactionLogsVisible} aria-controls="transaction-native-logs" on:click={() => transactionLogsVisible = !transactionLogsVisible}>{transactionLogsVisible ? 'Hide logs' : 'Show logs'}</button>{/if}</header>
      <div class="space-y-3 p-4">
        <p class="text-sm">{confirmation.description}</p>
        {#if transactionLoading}<div class="space-y-2" role="status" aria-live="polite"><p class="text-xs text-surface-500">{transactionPlan ? 'Applying transaction…' : 'Preparing transaction preview…'}</p><div class="h-1.5 overflow-hidden rounded-full bg-surface-300-700"><span class="block h-full w-1/3 animate-pulse rounded-full bg-primary-500"></span></div></div>{/if}
        {#if transactionPlan}
          <div class="overflow-hidden rounded-base border border-surface-300-700 bg-surface-50-900 text-xs"><p class="border-b border-surface-300-700 px-3 py-2 font-semibold">Package transaction preview</p>{#if transactionPlan.changes.length}<div class="max-h-64 overflow-auto"><table class="w-full table-fixed text-left"><thead class="sticky top-0 bg-surface-200-800"><tr><th class="w-[28%] px-3 py-2">Name</th><th class="w-[14%] px-2 py-2">Action</th><th class="w-[19%] px-2 py-2">Version</th><th class="w-[21%] px-2 py-2">New version</th><th class="w-[18%] px-3 py-2 text-right">Download size</th></tr></thead><tbody>{#each transactionPlan.changes as change}<tr class="border-t border-surface-200-800"><td class="truncate px-3 py-1.5 font-medium" title={change.name}>{change.name}</td><td class="px-2 py-1.5 capitalize">{change.action}</td><td class="truncate px-2 py-1.5 font-mono" title={change.version || '—'}>{change.version || '—'}</td><td class="truncate px-2 py-1.5 font-mono" title={change.newVersion || '—'}>{change.newVersion || '—'}</td><td class="px-3 py-1.5 text-right font-mono">{change.action === 'remove' ? '—' : formatBytes(change.downloadSize)}</td></tr>{/each}</tbody></table></div>{#if transactionPlan.operation === 'remove' || transactionPlan.operation === 'purge' || transactionPlan.operation === 'remove-orphans' || transactionPlan.operation === 'force-remove' || transactionPlan.operation === 'purge-kernels'}<dl class="grid grid-cols-[1fr_auto] gap-x-5 gap-y-1 border-t border-surface-300-700 px-3 py-2 font-mono"><dt>Size freed on disk</dt><dd class="text-right">{formatBytes(transactionPlan.installedSize)}</dd><dt>Space available on disk</dt><dd class="text-right">{formatBytes(transactionPlan.availableSpace)}</dd></dl>{:else if transactionPlan.operation === 'install' || transactionPlan.operation === 'update' || transactionPlan.operation === 'reinstall' || transactionPlan.operation === 'downgrade' || transactionPlan.operation === 'system-upgrade'}<dl class="grid grid-cols-[1fr_auto] gap-x-5 gap-y-1 border-t border-surface-300-700 px-3 py-2 font-mono"><dt>Size to download</dt><dd class="text-right">{formatBytes(transactionPlan.downloadSize)}</dd><dt>Size required on disk</dt><dd class="text-right">{formatBytes(transactionPlan.installedSize)}</dd><dt>Space available on disk</dt><dd class="text-right">{formatBytes(transactionPlan.availableSpace)}</dd></dl>{/if}{:else if transactionPlan.diagnostics.length}<ul class="max-h-40 overflow-auto p-3 font-mono">{#each transactionPlan.diagnostics as line}<li class="break-all py-0.5">{line}</li>{/each}</ul>{:else}<p class="p-3 text-surface-500">No package changes were reported by the native preview.</p>{/if}</div>
        {:else if !transactionLoading && !transactionError && !confirmationAction}
          <p class="rounded-base border border-warning-500/40 bg-warning-500/10 p-3 text-xs">This action is unavailable.</p>
        {/if}
        {#if transactionLogsVisible}<section id="transaction-native-logs" class="overflow-hidden rounded-base border border-surface-300-700 bg-surface-950 text-surface-50"><h3 class="border-b border-surface-700 px-3 py-2 text-xs font-semibold">Native logs</h3><pre class="max-h-48 overflow-auto whitespace-pre-wrap break-words p-3 font-mono text-xs">{transactionLogs.length ? transactionLogs.join('\n') : 'No native output.'}</pre></section>{/if}
        {#if transactionError}<p class="rounded-base border border-error-500/40 bg-error-500/10 p-3 text-xs" role="alert">{transactionError}</p>{/if}
        {#if transactionOutcome === 'failure'}<div class="flex items-center gap-3 rounded-base border border-warning-500/40 bg-warning-500/10 p-3 text-xs"><p>Package state may have changed. Reload the catalog before creating a new transaction preview.</p><button type="button" class="btn preset-tonal ml-auto shrink-0" on:click={() => { confirmationDialog.close(); reloadPackages(); }}>Reload state</button></div>{/if}
        {#if transactionOutcome === 'success'}<p class="rounded-base border border-success-500/40 bg-success-500/10 p-3 text-sm text-success-700-300" role="status">Transaction complete.</p>{/if}
      </div>
      <footer class="flex justify-end gap-2 border-t border-surface-300-700 px-4 py-3">
        <button type="submit" class="btn preset-tonal" disabled={transactionLoading}>{transactionOutcome ? 'Close' : 'Cancel'}</button>
        {#if !transactionOutcome}<button type="button" class="btn {confirmation.destructive ? 'bg-error-600 text-white' : 'preset-filled-primary-500'}" disabled={(!transactionPlan && !confirmationAction) || transactionLoading} on:click={executeConfirmation}>{transactionLoading ? 'Working…' : 'Confirm'}</button>{/if}
      </footer>
    </form>
  </dialog>

  <dialog bind:this={ruleDialog} class="m-auto w-[min(30rem,calc(100%-2rem))] rounded-base border border-surface-300-700 bg-surface-100-800 p-0 text-surface-950-50 shadow-xl backdrop:bg-black/45" aria-labelledby="rule-title">
    <form on:submit|preventDefault={saveRule}>
      <header class="border-b border-surface-300-700 px-4 py-3"><h2 id="rule-title" class="font-semibold">{ruleOperation === 'edit' ? 'Edit XBPS rule' : 'Add XBPS rule'}</h2></header>
      <div class="space-y-3 p-4">
        <label class="block text-sm"><span class="mb-1 block font-medium">Rule type</span><select class="select app-input-surface w-full" bind:value={ruleKind} disabled={ruleOperation === 'edit'}><option value="ignorepkg">Ignored package</option><option value="virtualpkg">Virtual package mapping</option></select></label>
        <label class="block text-sm"><span class="mb-1 block font-medium">Value</span><input class="input app-input-surface w-full font-mono" bind:value={ruleValue} placeholder={ruleKind === 'ignorepkg' ? 'package-name' : 'virtual-name:provider-package'} autocomplete="off" /></label>
        <p class="text-xs text-surface-500">Sysentra writes rules to <span class="font-mono">/etc/xbps.d/sysentra-rules.conf</span>.</p>
        {#if ruleError}<p class="rounded-base border border-error-500/40 bg-error-500/10 p-3 text-xs" role="alert">{ruleError}</p>{/if}
      </div>
      <footer class="flex justify-end gap-2 border-t border-surface-300-700 px-4 py-3"><button type="button" class="btn preset-tonal" disabled={ruleSaving} on:click={() => ruleDialog.close()}>Cancel</button><button type="submit" class="btn preset-filled-primary-500" disabled={ruleSaving || !ruleValue.trim()}>{ruleSaving ? 'Saving…' : ruleOperation === 'edit' ? 'Save' : 'Add rule'}</button></footer>
    </form>
  </dialog>

  <dialog bind:this={firewallDialog} class="m-auto w-[min(32rem,calc(100%-2rem))] rounded-base border border-surface-300-700 bg-surface-100-800 p-0 text-surface-950-50 shadow-xl backdrop:bg-black/45" aria-labelledby="firewall-dialog-title">
    <form on:submit|preventDefault={saveFirewallConfiguration}>
      <header class="border-b border-surface-300-700 px-4 py-3"><h2 id="firewall-dialog-title" class="font-semibold">{firewallDialogMode === 'policy' ? 'UFW default policy' : firewallDialogMode === 'zone' ? 'firewalld default zone' : firewallDialogMode === 'edit' ? 'Edit firewall rule' : 'Add firewall rule'}</h2></header>
      <div class="space-y-3 p-4">
        {#if firewallDialogMode === 'policy'}
          <div class="grid grid-cols-2 gap-3"><label class="block text-sm"><span class="mb-1 block font-medium">Direction</span><select class="select app-input-surface w-full" bind:value={firewallDirection}><option value="incoming">Incoming</option><option value="outgoing">Outgoing</option><option value="routed">Routed</option></select></label><label class="block text-sm"><span class="mb-1 block font-medium">Policy</span><select class="select app-input-surface w-full" bind:value={firewallPolicy}><option value="allow">Allow</option><option value="deny">Deny</option><option value="reject">Reject</option></select></label></div>
          <p class="text-xs text-surface-500">Restrictive incoming policies require an existing SSH allow rule.</p>
        {:else if firewallDialogMode === 'zone'}
          <label class="block text-sm"><span class="mb-1 block font-medium">Default zone</span><input class="input app-input-surface w-full" bind:value={firewallZone} maxlength="64" pattern="[A-Za-z0-9][A-Za-z0-9_.-]*" required /></label>
          <p class="text-xs text-surface-500">The selected zone must allow the SSH service. Sysentra verifies this before switching.</p>
        {:else}
          {#if firewall?.backend === 'firewalld'}<label class="block text-sm"><span class="mb-1 block font-medium">Rule type</span><select class="select app-input-surface w-full" bind:value={firewallRuleKind}><option value="port">Port</option><option value="service">Service</option></select></label>{/if}
          <div class="grid grid-cols-[1fr_8rem] gap-3"><label class="block text-sm"><span class="mb-1 block font-medium">{firewallRuleKind === 'service' ? 'Service' : 'Port'}</span><input class="input app-input-surface w-full" type={firewallRuleKind === 'port' ? 'number' : 'text'} min={firewallRuleKind === 'port' ? 1 : undefined} max={firewallRuleKind === 'port' ? 65535 : undefined} bind:value={firewallTarget} placeholder={firewallRuleKind === 'service' ? 'https' : '443'} required /></label>{#if firewallRuleKind === 'port'}<label class="block text-sm"><span class="mb-1 block font-medium">Protocol</span><select class="select app-input-surface w-full" bind:value={firewallProtocol}><option value="tcp">TCP</option><option value="udp">UDP</option></select></label>{/if}</div>
          {#if firewall?.backend === 'ufw'}<div class="grid grid-cols-[1fr_7rem_1.2fr] gap-3"><label class="block text-sm"><span class="mb-1 block font-medium">Action</span><select class="select app-input-surface w-full" bind:value={firewallAction}><option value="allow">Allow</option><option value="deny">Deny</option><option value="reject">Reject</option><option value="limit">Rate limit</option></select></label><label class="block text-sm"><span class="mb-1 block font-medium">Direction</span><select class="select app-input-surface w-full" bind:value={firewallRuleDirection}><option value="in">In</option><option value="out">Out</option></select></label><label class="block text-sm"><span class="mb-1 block font-medium">Source</span><input class="input app-input-surface w-full font-mono" bind:value={firewallSource} placeholder="any or 10.0.0.0/8" required /></label></div>{/if}
          <p class="text-xs text-surface-500">Rules are applied through {firewall?.backend}. SSH on port 22 is protected from editing or removal.</p>
        {/if}
        {#if firewallError}<p class="rounded-base border border-error-500/40 bg-error-500/10 p-3 text-xs" role="alert">{firewallError}</p>{/if}
      </div>
      <footer class="flex justify-end gap-2 border-t border-surface-300-700 px-4 py-3"><button type="button" class="btn preset-tonal" disabled={firewallSaving} on:click={() => firewallDialog.close()}>Cancel</button><button type="submit" class="btn preset-filled-primary-500" disabled={firewallSaving || (firewallDialogMode === 'zone' ? !firewallZone.trim() : firewallDialogMode === 'policy' ? false : !firewallTarget.trim())}>{firewallSaving ? 'Saving…' : 'Apply'}</button></footer>
    </form>
  </dialog>

  <dialog bind:this={firmwareDialog} class="m-auto w-[min(42rem,calc(100%-2rem))] rounded-base border border-surface-300-700 bg-surface-100-800 p-0 text-surface-950-50 shadow-xl backdrop:bg-black/45" aria-labelledby="firmware-dialog-title">
    <div>
      <header class="border-b border-surface-300-700 px-4 py-3"><h2 id="firmware-dialog-title" class="font-semibold">{firmwareOperation === 'refresh' ? 'Refresh firmware metadata' : `Update ${firmwareDevice?.name || 'firmware'}`}</h2></header>
      <div class="space-y-3 p-4">
        {#if firmwareOperation === 'refresh'}
          <p class="text-sm">Download current metadata from enabled fwupd remotes.</p>
        {:else if firmwareDevice}
          <dl class="grid grid-cols-[8rem_minmax(0,1fr)] gap-x-3 gap-y-1 text-sm"><dt class="text-surface-500">Version</dt><dd class="font-mono">{firmwareDevice.version || '—'} → {firmwareDevice.newVersion}</dd><dt class="text-surface-500">Source</dt><dd>{firmwareDevice.updateRemote || 'fwupd remote'}</dd><dt class="text-surface-500">Urgency</dt><dd class="capitalize">{firmwareDevice.updateUrgency || 'Unspecified'}</dd><dt class="text-surface-500">Download</dt><dd>{firmwareDevice.updateSize ? formatBytes(firmwareDevice.updateSize) : 'Not reported'}</dd></dl>
          {#if firmwareDevice.updateSummary}<p class="text-sm">{firmwareDevice.updateSummary}</p>{/if}
          {#if firmwareDevice.updateMessage}<p class="rounded-base border border-warning-500/40 bg-warning-500/10 p-3 text-sm">{firmwareDevice.updateMessage}</p>{/if}
          <div class="flex flex-wrap gap-2 text-xs">{#if firmwareDevice.acRequired}<span class="rounded-full border border-warning-500/40 bg-warning-500/10 px-2 py-1">Connect AC power</span>{/if}{#if firmwareDevice.shutdownRequired}<span class="rounded-full border border-warning-500/40 bg-warning-500/10 px-2 py-1">Shutdown required</span>{:else if firmwareDevice.rebootRequired}<span class="rounded-full border border-warning-500/40 bg-warning-500/10 px-2 py-1">Reboot required</span>{/if}</div>
          <p class="text-xs text-surface-500">fwupd keeps its signature, power, device-safety, and offline-update checks enabled.</p>
        {/if}
        {#if firmwareActionLoading}<div class="space-y-2" role="status" aria-live="polite"><p class="text-sm">{firmwareOperation === 'refresh' ? 'Refreshing metadata…' : 'Applying firmware update…'}</p><div class="h-1.5 overflow-hidden rounded-full bg-surface-300-700"><span class="block h-full w-1/3 animate-pulse rounded-full bg-primary-500"></span></div></div>{/if}
        {#if firmwareActionLogs.length}<section class="overflow-hidden rounded-base border border-surface-300-700 bg-surface-950 text-surface-50" aria-label="fwupd output"><pre class="max-h-48 overflow-auto whitespace-pre-wrap break-words p-3 font-mono text-xs">{firmwareActionLogs.join('\n')}</pre></section>{/if}
        {#if firmwareActionError}<p class="rounded-base border border-error-500/40 bg-error-500/10 p-3 text-sm text-error-700-300" role="alert">{firmwareActionError}</p>{:else if firmwareActionSucceeded}<p class="rounded-base border border-success-500/40 bg-success-500/10 p-3 text-sm text-success-700-300" role="status">{firmwareOperation === 'refresh' ? 'Firmware metadata refreshed.' : firmwareDevice?.shutdownRequired ? 'Firmware staged. Shut down when ready to complete the update.' : firmwareDevice?.rebootRequired ? 'Firmware staged. Reboot when ready to complete the update.' : 'Firmware update completed.'}</p>{/if}
      </div>
      <footer class="flex justify-end gap-2 border-t border-surface-300-700 px-4 py-3"><button type="button" class="btn preset-tonal" disabled={firmwareActionLoading} on:click={() => firmwareDialog.close()}>{firmwareActionSucceeded ? 'Close' : 'Cancel'}</button>{#if !firmwareActionSucceeded}<button type="button" class="btn preset-filled-primary-500" disabled={firmwareActionLoading || (firmwareOperation === 'update' && !firmwareDevice?.newVersion)} on:click={executeFirmwareAction}>{firmwareActionLoading ? 'Working…' : firmwareOperation === 'refresh' ? 'Refresh' : 'Install update'}</button>{/if}</footer>
    </div>
  </dialog>

  <dialog bind:this={accountDialog} class="m-auto w-[min(30rem,calc(100%-2rem))] rounded-base border border-surface-300-700 bg-surface-100-800 p-0 text-surface-950-50 shadow-xl backdrop:bg-black/45" aria-labelledby="account-title">
    <form on:submit|preventDefault={saveAccount}>
      <header class="border-b border-surface-300-700 px-4 py-3"><h2 id="account-title" class="font-semibold">{accountOperation === 'user-add' ? 'Add user' : accountOperation === 'group-add' ? 'Add group' : accountOperation === 'user-edit' ? `Edit ${accountUser}` : accountOperation === 'primary-group-change' ? `Primary group for ${accountUser}` : accountOperation === 'membership-add' ? `Assign group to ${accountUser}` : `Unassign group from ${accountUser}`}</h2></header>
      <div class="space-y-3 p-4">
        {#if accountOperation === 'user-add'}<label class="block text-sm"><span class="mb-1 block font-medium">User name</span><input class="input app-input-surface w-full" bind:value={accountUser} pattern="[a-z_][a-z0-9_-]*" maxlength="32" autocomplete="off" required /></label><label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={accountCreateHome} /> Create home directory</label>{#if accountCreateHome}<label class="block text-sm"><span class="mb-1 block font-medium">Custom home path <span class="font-normal text-surface-500">(optional)</span></span><input class="input app-input-surface w-full font-mono" bind:value={accountHome} placeholder={`/home/${accountUser || 'username'}`} /></label>{/if}<p class="text-xs text-surface-500">Sysentra creates a matching primary group. Leave the path empty to use the default home.</p>{:else if accountOperation === 'group-add'}<label class="block text-sm"><span class="mb-1 block font-medium">Group name</span><input class="input app-input-surface w-full" bind:value={accountGroup} pattern="[a-z_][a-z0-9_-]*" maxlength="32" autocomplete="off" required /></label>{:else if accountOperation === 'user-edit'}<label class="block text-sm"><span class="mb-1 block font-medium">Home directory</span><input class="input app-input-surface w-full font-mono" bind:value={accountHome} required /></label><label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={accountMoveHome} /> Move existing home contents</label><label class="block text-sm"><span class="mb-1 block font-medium">Login shell</span><input class="input app-input-surface w-full font-mono" bind:value={accountShell} required /></label>{:else if accountOperation === 'primary-group-change'}<label class="block text-sm"><span class="mb-1 block font-medium">Primary group</span><select class="select app-input-surface w-full" bind:value={accountGroup} required>{#each groups as group}<option value={group.name}>{group.name}</option>{/each}</select></label>{:else if accountOperation === 'membership-add'}<fieldset><legend class="mb-1 text-sm font-medium">Groups</legend><div class="max-h-60 overflow-auto rounded-base border border-surface-300-700 p-1">{#each groups.filter((group) => !users.find((user) => user.name === accountUser)?.groups.includes(group.name)) as group}<label class="flex cursor-pointer items-center gap-2 rounded-base px-2 py-1.5 text-sm hover:bg-surface-200-800"><input type="checkbox" checked={accountGroups.includes(group.name)} on:change={() => toggleAccountGroup(group.name)} /><span>{group.name}</span><span class="ml-auto font-mono text-xs text-surface-500">{group.gid}</span></label>{:else}<p class="p-2 text-sm text-surface-500">This user belongs to all listed groups.</p>{/each}</div><p class="mt-1 text-xs text-surface-500">Select supplementary groups.</p></fieldset>{:else}<label class="block text-sm"><span class="mb-1 block font-medium">Group</span><select class="select app-input-surface w-full" bind:value={accountGroup} required><option value="" disabled>Select group</option>{#each groups.filter((group) => users.find((user) => user.name === accountUser)?.groups.includes(group.name) && group.gid !== users.find((user) => user.name === accountUser)?.gid) as group}<option value={group.name}>{group.name}</option>{/each}</select></label>{/if}
        {#if accountError}<p class="rounded-base border border-error-500/40 bg-error-500/10 p-3 text-xs" role="alert">{accountError}</p>{/if}
      </div>
      <footer class="flex justify-end gap-2 border-t border-surface-300-700 px-4 py-3"><button type="button" class="btn preset-tonal" disabled={accountSaving} on:click={() => accountDialog.close()}>Cancel</button><button type="submit" class="btn preset-filled-primary-500" disabled={accountSaving || (accountOperation === 'user-add' ? !accountUser.trim() : accountOperation === 'user-edit' ? !accountHome.trim() || !accountShell.trim() : accountOperation === 'membership-add' ? accountGroups.length === 0 : !accountGroup.trim())}>{accountSaving ? 'Saving…' : 'Confirm'}</button></footer>
    </form>
  </dialog>

  <dialog bind:this={accountHistoryDialog} class="m-auto w-[min(42rem,calc(100%-2rem))] rounded-base border border-surface-300-700 bg-surface-100-800 p-0 text-surface-950-50 shadow-xl backdrop:bg-black/45" aria-labelledby="account-history-title">
    <header class="flex items-center border-b border-surface-300-700 px-4 py-3"><h2 id="account-history-title" class="font-semibold">Account history</h2><button type="button" class="btn preset-tonal ml-auto" on:click={() => accountHistoryDialog.close()}>Close</button></header>
    <div class="max-h-[28rem] overflow-auto"><div class="grid grid-cols-[11rem_1fr_1fr] border-b border-surface-300-700 bg-surface-50-900 px-4 py-2 text-xs font-semibold"><span>Time</span><span>Action</span><span>Target</span></div>{#each accountHistory as item}<div class="grid grid-cols-[11rem_1fr_1fr] border-b border-surface-200-800 px-4 py-2 text-sm"><span class="font-mono text-xs">{new Date(item.time).toLocaleString()}</span><span>{item.operation}</span><span>{item.target}</span></div>{:else}<p class="p-5 text-sm text-surface-500">No account changes recorded.</p>{/each}</div>
  </dialog>

  <dialog bind:this={sambaGlobalDialog} class="m-auto w-[min(34rem,calc(100%-2rem))] rounded-base border border-surface-300-700 bg-surface-100-800 p-0 text-surface-950-50 shadow-xl backdrop:bg-black/45" aria-labelledby="samba-global-title">
    <form on:submit|preventDefault={saveSambaGlobal}>
      <header class="border-b border-surface-300-700 px-4 py-3"><h2 id="samba-global-title" class="font-semibold">Global Samba configuration</h2></header>
      <div class="space-y-4 p-4">
        <label class="block text-sm"><span class="mb-1 block font-medium">Samba config path</span><input class="input app-input-surface w-full font-mono" value={sambaServer?.global.configPath ?? '/etc/samba/smb.conf'} readonly /></label>
        <label class="block text-sm"><span class="mb-1 block font-medium">Minimum SMB protocol</span><select class="select app-input-surface w-full" bind:value={sambaGlobalProtocol}><option value="SMB2">SMB2 (recommended)</option><option value="SMB3">SMB3</option></select></label>
        <label class="block text-sm"><span class="mb-1 block font-medium">Default permissions</span><select class="select app-input-surface w-full" bind:value={sambaGlobalMask}>{#each sambaPermissions as permission}<option value={permission[0]}>{permission[1]}</option>{/each}</select></label>
        <label class="flex items-start gap-3 text-sm"><input class="mt-1" type="checkbox" role="switch" bind:checked={sambaGlobalPrinting} /><span><strong class="block">Samba printing (CUPS)</strong><span class="text-xs text-surface-500">Adds managed CUPS settings and missing printer shares.</span></span></label>
        <label class="flex items-start gap-3 text-sm"><input class="mt-1" type="checkbox" role="switch" bind:checked={sambaGlobalMapGuest} /><span><strong class="block">Map unknown users to guest</strong><span class="text-xs text-surface-500">Uses <span class="font-mono">map to guest = Bad User</span>; otherwise uses <span class="font-mono">Never</span>.</span></span></label>
        <div class="flex items-start gap-3 text-sm"><input class="mt-1" type="checkbox" role="switch" checked={sambaServer?.global.firewallActive ?? false} disabled /><span><strong class="block">System firewall</strong><span class="text-xs text-surface-500">{sambaServer?.global.firewallBackend ? `${sambaServer.global.firewallBackend} is ${sambaServer.global.firewallActive ? 'active' : 'inactive'}.` : 'No supported firewall detected.'}</span></span></div>
        <label class="flex items-start gap-3 text-sm"><input class="mt-1" type="checkbox" role="switch" bind:checked={sambaGlobalFirewallAllowed} disabled={!sambaServer?.global.firewallActive} /><span><strong class="block">Allow Samba through the firewall</strong><span class="text-xs text-surface-500">{sambaServer?.global.firewallActive ? 'Manages the Samba service or ports 137–139 and 445 in the active backend.' : 'Enable and select a firewall backend in the Firewall section first.'}</span></span></label>
        {#if sambaGlobalError}<p class="rounded-base border border-error-500/40 bg-error-500/10 p-3 text-xs" role="alert">{sambaGlobalError}</p>{/if}
      </div>
      <footer class="flex justify-end gap-2 border-t border-surface-300-700 px-4 py-3"><button type="button" class="btn preset-tonal" disabled={sambaGlobalSaving} on:click={() => sambaGlobalDialog.close()}>Cancel</button><button type="submit" class="btn preset-filled-primary-500" disabled={sambaGlobalSaving}>{sambaGlobalSaving ? 'Saving…' : 'Save'}</button></footer>
    </form>
  </dialog>

  <dialog bind:this={configDialog} class="m-auto w-[min(46rem,calc(100%-2rem))] rounded-base border border-surface-300-700 bg-surface-100-800 p-0 text-surface-950-50 shadow-xl backdrop:bg-black/45" aria-labelledby="config-title">
    <form on:submit|preventDefault={saveConfig}>
      <header class="border-b border-surface-300-700 px-4 py-3"><h2 id="config-title" class="font-semibold">{configOperation === 'edit' ? `Edit ${configKind === 'flatpak' ? 'Flatpak remote' : configKind === 'samba' ? 'Samba share' : configKind === 'nfs' ? 'NFS mount' : 'repository'}` : configKind === 'repository' ? 'Add repository' : configKind === 'flatpak' ? 'Add Flatpak remote' : configKind === 'samba' ? 'Add Samba share' : 'Add NFS mount'}</h2></header>
      <div class="max-h-[calc(100vh-10rem)] space-y-3 overflow-y-auto p-4">
        {#if configKind === 'flatpak'}<label class="block text-sm"><span class="mb-1 block font-medium">Remote name</span><input class="input app-input-surface w-full" bind:value={configName} placeholder="flathub" disabled={configOperation === 'edit'} required /></label>{/if}
        <label class="block text-sm"><span class="mb-1 block font-medium">{configKind === 'repository' || configKind === 'flatpak' ? 'HTTPS repository URL' : configKind === 'samba' ? 'Share name' : 'NFS source'}</span><input class="input app-input-surface w-full" bind:value={configValue} on:input={() => { configTrustConfirmed = false; configRepositoryKey = null; configKeyURL = ''; }} placeholder={configKind === 'repository' ? 'https://repo.example.org/current' : configKind === 'flatpak' ? 'https://flathub.org/repo/flathub.flatpakrepo' : configKind === 'samba' ? 'Media' : 'server:/data'} required /></label>
        {#if configKind === 'samba' || configKind === 'nfs'}<label class="block text-sm"><span class="mb-1 block font-medium">{configKind === 'samba' ? 'Local directory' : 'Mount point'}</span><input class="input app-input-surface w-full font-mono" bind:value={configPath} placeholder={configKind === 'samba' ? '/srv/media' : '/mnt/data'} required /></label>{/if}
        {#if configKind === 'samba'}
          <label class="block text-sm"><span class="mb-1 block font-medium">Comment</span><input class="input app-input-surface w-full" bind:value={configComment} maxlength="256" placeholder="Shared media" /></label>
          <fieldset class="grid grid-cols-2 gap-2 rounded-base border border-surface-300-700 p-3 text-sm sm:grid-cols-3"><legend class="px-1 font-medium">Access behavior</legend><label class="flex items-center gap-2"><input type="checkbox" bind:checked={configBrowseable} /> Browseable</label><label class="flex items-center gap-2"><input type="checkbox" bind:checked={configReadOnly} /> Read only</label><label class="flex items-center gap-2"><input type="checkbox" bind:checked={configGuestOK} /> Allow guests</label><label class="flex items-center gap-2"><input type="checkbox" bind:checked={configFollowSymlinks} /> Follow symlinks</label><label class="flex items-center gap-2"><input type="checkbox" bind:checked={configWideLinks} /> Wide links</label></fieldset>
          {#if configWideLinks}<p class="rounded-base border border-warning-500/40 bg-warning-500/10 p-2 text-xs text-warning-700-300">Wide links can expose files outside the shared directory. Samba may also require compatible global settings.</p>{/if}
          <div class="grid gap-3 sm:grid-cols-2"><label class="block text-sm"><span class="mb-1 block font-medium">Valid users</span><input class="input app-input-surface w-full" bind:value={configValidUsers} placeholder="alice, bob" /></label><details class="rounded-base border border-surface-300-700"><summary class="cursor-pointer px-3 py-2 text-sm font-medium">Valid groups ({configValidGroups.length})</summary><div class="max-h-36 overflow-auto border-t border-surface-300-700 p-1">{#each groups as group}<label class="flex cursor-pointer items-center gap-2 px-2 py-1 text-sm"><input type="checkbox" checked={configValidGroups.includes(group.name)} on:change={() => toggleSambaGroup('valid', group.name)} /> {group.name}</label>{:else}<p class="p-2 text-xs text-surface-500">No local groups available.</p>{/each}</div></details><label class="block text-sm"><span class="mb-1 block font-medium">Write list</span><input class="input app-input-surface w-full" bind:value={configWriteList} placeholder="alice" /></label><details class="rounded-base border border-surface-300-700"><summary class="cursor-pointer px-3 py-2 text-sm font-medium">Write groups ({configWriteGroups.length})</summary><div class="max-h-36 overflow-auto border-t border-surface-300-700 p-1">{#each groups as group}<label class="flex cursor-pointer items-center gap-2 px-2 py-1 text-sm"><input type="checkbox" checked={configWriteGroups.includes(group.name)} on:change={() => toggleSambaGroup('write', group.name)} /> {group.name}</label>{/each}</div></details><label class="block text-sm"><span class="mb-1 block font-medium">Read list</span><input class="input app-input-surface w-full" bind:value={configReadList} placeholder="bob" /></label><details class="rounded-base border border-surface-300-700"><summary class="cursor-pointer px-3 py-2 text-sm font-medium">Read groups ({configReadGroups.length})</summary><div class="max-h-36 overflow-auto border-t border-surface-300-700 p-1">{#each groups as group}<label class="flex cursor-pointer items-center gap-2 px-2 py-1 text-sm"><input type="checkbox" checked={configReadGroups.includes(group.name)} on:change={() => toggleSambaGroup('read', group.name)} /> {group.name}</label>{/each}</div></details></div>
          <div class="grid gap-3 sm:grid-cols-2"><label class="block text-sm"><span class="mb-1 block font-medium">Force user</span><input class="input app-input-surface w-full" list="samba-users" bind:value={configForceUser} /></label><label class="block text-sm"><span class="mb-1 block font-medium">Force group</span><input class="input app-input-surface w-full" list="samba-groups" bind:value={configForceGroup} /></label><label class="block text-sm"><span class="mb-1 block font-medium">Create mask</span><select class="select app-input-surface w-full" bind:value={configCreateMask}>{#each sambaPermissions as permission}<option value={permission[0]}>{permission[1]}</option>{/each}</select></label><label class="block text-sm"><span class="mb-1 block font-medium">Directory mask</span><select class="select app-input-surface w-full" bind:value={configDirectoryMask}>{#each sambaPermissions as permission}<option value={permission[0]}>{permission[1]}</option>{/each}</select></label></div>
          <datalist id="samba-users">{#each users as user}<option value={user.name}></option>{/each}</datalist><datalist id="samba-groups">{#each groups as group}<option value={group.name}></option>{/each}</datalist>
        {/if}
        {#if configKind === 'nfs'}<div class="grid grid-cols-[7rem_1fr] gap-3"><label class="block text-sm"><span class="mb-1 block font-medium">Type</span><select class="select app-input-surface w-full" bind:value={configType}><option value="nfs4">NFS 4</option><option value="nfs">NFS</option></select></label><label class="block text-sm"><span class="mb-1 block font-medium">Options</span><input class="input app-input-surface w-full font-mono" bind:value={configOptions} /></label></div>{/if}
        {#if configKind === 'repository'}<div class="space-y-2"><button type="button" class="btn preset-tonal" disabled={configKeyLoading || !configValue.trim()} on:click={inspectRepositoryKey}>{configKeyLoading ? 'Inspecting…' : 'Inspect signing key'}</button>{#if configRepositoryKey && configKeyURL === configValue.trim()}<dl class="grid grid-cols-[7rem_minmax(0,1fr)] gap-x-3 gap-y-1 rounded-base border border-surface-300-700 bg-surface-50-900 p-3 text-xs"><dt class="text-surface-500">Signer</dt><dd>{configRepositoryKey.signer}</dd><dt class="text-surface-500">Key</dt><dd>{configRepositoryKey.type.toUpperCase()} {configRepositoryKey.bits}-bit</dd><dt class="text-surface-500">Signing key fingerprint</dt><dd class="break-all font-mono">{configRepositoryKey.fingerprint}</dd></dl>{/if}<label class="flex items-start gap-2 text-sm"><input class="mt-0.5" type="checkbox" bind:checked={configTrustConfirmed} disabled={!configRepositoryKey || configKeyURL !== configValue.trim()} /><span>I verified this signing identity and trust packages from this repository.</span></label></div>{/if}
        {#if configKind === 'repository'}<p class="text-xs text-surface-500">Sysentra needs administrator authorization to update this configuration.</p>{/if}
        {#if configError}<p class="rounded-base border border-error-500/40 bg-error-500/10 p-3 text-xs" role="alert">{configError}</p>{/if}
      </div>
      <footer class="flex justify-end gap-2 border-t border-surface-300-700 px-4 py-3"><button type="button" class="btn preset-tonal" disabled={configSaving} on:click={() => configDialog.close()}>Cancel</button><button type="submit" class="btn preset-filled-primary-500" disabled={configSaving || !configValue.trim() || (configKind === 'repository' && !configTrustConfirmed) || (configKind === 'flatpak' && !configName.trim()) || ((configKind === 'samba' || configKind === 'nfs') && !configPath.trim())}>{configSaving ? 'Saving…' : 'Save'}</button></footer>
    </form>
  </dialog>

  <dialog bind:this={ownerDialog} class="m-auto w-[min(34rem,calc(100%-2rem))] rounded-base border border-surface-300-700 bg-surface-100-800 p-0 text-surface-950-50 shadow-xl backdrop:bg-black/45" aria-labelledby="owner-title">
    <div>
      <header class="border-b border-surface-300-700 px-4 py-3"><h2 id="owner-title" class="font-semibold">Find owning package</h2><p class="text-xs text-surface-500">Enter a file path or an XBPS-compatible pattern.</p></header>
      <div class="space-y-3 p-4">
        <form class="flex gap-2" on:submit|preventDefault={findOwningPackage}><label for="owner-query" class="sr-only">File path or pattern</label><input id="owner-query" class="input app-input-surface min-w-0 flex-1" placeholder="/usr/bin/bash" bind:value={ownerQuery} /><button type="submit" class="btn preset-filled-primary-500" disabled={ownerLoading || !ownerQuery.trim()}>{ownerLoading ? 'Searching…' : 'Search'}</button></form>
        {#if ownerError}<p class="rounded-base border border-error-500/40 bg-error-500/10 p-3 text-sm" role="alert">{ownerError}</p>{/if}
        {#if ownerSearched && !ownerError && ownerMatches.length === 0}<p class="text-sm text-surface-500" role="status">No installed package owns a matching file.</p>{/if}
        {#if ownerMatches.length > 0}<ul class="max-h-64 overflow-auto rounded-base border border-surface-300-700 font-mono text-xs">{#each ownerMatches as match}<li class="border-b border-surface-200-800 p-2 last:border-0">{match}</li>{/each}</ul>{/if}
      </div>
      <footer class="flex justify-end border-t border-surface-300-700 px-4 py-3"><button type="button" class="btn preset-tonal" on:click={() => ownerDialog.close()}>Close</button></footer>
    </div>
  </dialog>
</div>
