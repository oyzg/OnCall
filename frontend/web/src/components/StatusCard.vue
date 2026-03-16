<template>
  <article class="status-card">
    <div class="status-card__header">
      <span>{{ title }}</span>
      <strong>{{ value }}</strong>
    </div>
    <p>{{ description }}</p>
  </article>
</template>

<script setup lang="ts">
defineProps<{
  title: string;
  value: string;
  description: string;
}>();
</script>
