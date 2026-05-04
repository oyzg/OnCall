<template>
  <article class="status-card">
    <div class="status-card__header">
      <span>{{ title }}</span>
      <strong>{{ value }}</strong>
    </div>
    <p>{{ description }}</p>
    <ul v-if="details?.length" class="status-card__details">
      <li v-for="detail in details" :key="detail">{{ detail }}</li>
    </ul>
  </article>
</template>

<script setup lang="ts">
defineProps<{
  title: string;
  value: string;
  description: string;
  details?: string[];
}>();
</script>

<style scoped>
.status-card__details {
  margin: 12px 0 0;
  padding-left: 18px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.6;
}
</style>
