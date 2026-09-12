<script lang="ts">
  import SettingsView from '../../lib/components/views/SettingsView.svelte';
  import LastFMSettingsPanel from './LastFMSettingsPanel.svelte';
</script>

<SettingsView>
  <LastFMSettingsPanel slot="product-integrations" />
</SettingsView>
