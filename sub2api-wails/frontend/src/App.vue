<script setup lang="ts">
import { RouterView } from 'vue-router'
import Toast from '@/components/common/Toast.vue'
import NavigationProgress from '@/components/common/NavigationProgress.vue'
</script>

<template>
  <NavigationProgress />
  <RouterView />
  <Toast />
</template>
