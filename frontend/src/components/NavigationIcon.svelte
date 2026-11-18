<script lang="ts">
  export let name: string;
</script>

<svg viewBox="0 0 24 24" class="size-4 shrink-0" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
  {#if name === 'Discover'}
    <circle cx="12" cy="12" r="8.5"/><path d="m15.5 8.5-2.1 4.9-4.9 2.1 2.1-4.9Z"/>
  {:else if name === 'Installed'}
    <path d="M5 8.5 12 5l7 3.5v8L12 20l-7-3.5Z"/><path d="m8.5 12 2.2 2.2 4.8-4.8"/>
  {:else if name === 'Updates'}
    <path d="M19 8a7.5 7.5 0 1 0 .3 7.5M19 4v4h-4"/><path d="M12 8v4l2.5 1.5"/>
  {:else if name === 'Repositories'}
    <ellipse cx="12" cy="6" rx="7" ry="3"/><path d="M5 6v6c0 1.7 3.1 3 7 3s7-1.3 7-3V6M5 12v6c0 1.7 3.1 3 7 3s7-1.3 7-3v-6"/>
  {:else if name === 'XBPS Config'}
    <path d="M6 4h12v16H6zM9 8h6M9 12h6M9 16h3"/>
  {:else if name === 'Mirrors'}
    <circle cx="12" cy="12" r="8.5"/><path d="M3.5 12h17M12 3.5c2.3 2.3 3.5 5.1 3.5 8.5S14.3 18.2 12 20.5C9.7 18.2 8.5 15.4 8.5 12S9.7 5.8 12 3.5Z"/>
  {:else if name === 'Services'}
    <path d="M4 7h10M4 17h16M17 7h3M4 12h4M11 12h9"/><circle cx="15.5" cy="7" r="1.5"/><circle cx="9.5" cy="12" r="1.5"/>
  {:else if name === 'Users & Groups'}
    <circle cx="9" cy="8" r="3"/><path d="M3.5 19c.5-3.6 2.3-5.5 5.5-5.5s5 1.9 5.5 5.5M15.5 5.5a3 3 0 0 1 0 5.8M16 14c2.6.4 4 2 4.5 5"/>
  {:else if name === 'Firewall'}
    <path d="M12 3.5 19 6v5.5c0 4.2-2.4 7.2-7 9-4.6-1.8-7-4.8-7-9V6Z"/><path d="M8 9h8M8 13h8M10 9v4M14 9v4"/>
  {:else if name === 'Samba Server'}
    <rect x="4" y="5" width="16" height="6" rx="1.5"/><rect x="4" y="13" width="16" height="6" rx="1.5"/><path d="M8 8h.01M8 16h.01M16 8h1M16 16h1"/>
  {:else if name === 'NFS Client'}
    <path d="M4 7h6l2 2h8v10H4Z"/><path d="M8 14h8M13.5 11.5 16 14l-2.5 2.5"/>
  {:else if name === 'Kernels'}
    <rect x="5" y="5" width="14" height="14" rx="2"/><path d="M9 9h6v6H9ZM9 2.5v2.5M15 2.5v2.5M9 19v2.5M15 19v2.5M2.5 9H5M19 9h2.5M2.5 15H5M19 15h2.5"/>
  {:else if name === 'Firmware'}
    <path d="M12 3v9M8.5 8.5A6 6 0 1 0 15.5 8.5"/><path d="M9 18h6"/>
  {:else if name === 'Drivers'}
    <path d="m14.5 5.5 4 4-9 9H5.5v-4Z"/><path d="m12.5 7.5 4 4M5 5l3 3M16 16l3 3"/>
  {:else}
    <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2.8 2.8-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.6v.2h-4V21a1.7 1.7 0 0 0-1-1.6 1.7 1.7 0 0 0-1.9.3l-.1.1L4.2 17l.1-.1a1.7 1.7 0 0 0 .3-1.9A1.7 1.7 0 0 0 3 14H2.8v-4H3a1.7 1.7 0 0 0 1.6-1 1.7 1.7 0 0 0-.3-1.9L4.2 7 7 4.2l.1.1a1.7 1.7 0 0 0 1.9.3A1.7 1.7 0 0 0 10 3V2.8h4V3a1.7 1.7 0 0 0 1 1.6 1.7 1.7 0 0 0 1.9-.3l.1-.1L19.8 7l-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.6 1h.2v4H21a1.7 1.7 0 0 0-1.6 1Z"/>
  {/if}
</svg>
