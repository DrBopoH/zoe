<script setup>
import UiIcon from './ui/UiIcon.vue'

// Подсветка краёв экрана во время перетаскивания: чем ближе курсор к краю — тем ярче зона и тем быстрее едет страница.
defineProps({ edge: { type: Object, required: true } })
</script>

<template>
  <div class="ui-edge ui-edge--top" :style="{ top: edge.headerBottom + 'px', opacity: edge.top }" aria-hidden="true">
    <UiIcon name="arrowUp" size="lg" class="ui-edge__arrow" :style="{ transform: `translateY(${(1 - edge.top) * 10}px)` }" />
  </div>
  <div class="ui-edge ui-edge--bottom" :style="{ opacity: edge.bottom }" aria-hidden="true">
    <UiIcon name="arrowDown" size="lg" class="ui-edge__arrow" :style="{ transform: `translateY(${-(1 - edge.bottom) * 10}px)` }" />
  </div>
</template>
