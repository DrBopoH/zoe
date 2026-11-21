<script setup>
import UiEmpty from '../../components/ui/UiEmpty.vue'

// Страница дела «Glovo». Пока пустая: сюда лягут виды дохода (чаевые наличными, налогооблагаемый приход, зарплата на счёт ФОП),
// недельная записка номинального дохода, расходы и убытки. Шапку с названием и обложкой рисует WorkspaceView.
defineProps({
  workspace: { type: Object, required: true }
})
</script>

<template>
  <UiEmpty>Страница Glovo пока пустая: здесь будут доходы, расходы и убытки по делу</UiEmpty>
</template>
