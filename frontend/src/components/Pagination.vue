<template>
  <div v-if="totalPages > 1" class="pagination">
    <button type="button" :disabled="page <= 1" @click="go(1)">首页</button>
    <button type="button" :disabled="page <= 1" @click="go(page - 1)">上一页</button>
    <span class="pager-info">第 {{ page }} / {{ totalPages }} 页，共 {{ total }} 条</span>
    <button type="button" :disabled="page >= totalPages" @click="go(page + 1)">下一页</button>
    <button type="button" :disabled="page >= totalPages" @click="go(totalPages)">尾页</button>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  page: { type: Number, default: 1 },
  pageSize: { type: Number, default: 20 },
  total: { type: Number, default: 0 },
})
const emit = defineEmits(['change'])

const totalPages = computed(() => Math.max(1, Math.ceil((props.total || 0) / (props.pageSize || 1))))

function go(p) {
  if (p >= 1 && p <= totalPages.value && p !== props.page) emit('change', p)
}
</script>
