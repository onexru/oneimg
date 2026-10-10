<template>
  <!-- A scope change destroys selections, dialogs, observers and pending requests. -->
  <Gallery :key="management ? 'all' : 'mine'" :management="management" />
</template>
<script setup>
import Gallery from '@/views/Gallery.vue';
defineProps({ management: { type: Boolean, default: false } });
</script>
