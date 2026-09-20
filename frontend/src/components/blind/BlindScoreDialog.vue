<template>
  <el-dialog :model-value="modelValue" title="匿名评分（仅可提交一次）" width="460px" @update:model-value="onVisibleChange">
    <el-form label-width="92px">
      <el-form-item v-for="dim in DIMENSIONS" :key="dim.key" :label="dim.label" required>
        <el-input-number
          v-model="form[dim.key]"
          :min="BLIND_MIN_SCORE"
          :max="BLIND_MAX_SCORE"
          :step="0.5"
          :precision="1"
          controls-position="right"
        />
        <span class="hint">0 - 10 分</span>
      </el-form-item>
      <el-alert type="warning" :closable="false" show-icon title="提交后不可修改；在揭晓前你只能看到自己的评分。" />
    </el-form>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submit">提交评分</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { submitBlindScore } from '@/api/blind'
import type { BlindScorePayload } from '@/constants/blind'
import { BLIND_MAX_SCORE, BLIND_MIN_SCORE } from '@/constants/blind'

const props = defineProps<{ modelValue: boolean; sessionId: number | string }>()
const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void
  (e: 'submitted'): void
}>()

const DIMENSIONS = [
  { key: 'aroma_score', label: '香气' },
  { key: 'acidity_score', label: '酸质' },
  { key: 'body_score', label: '醇厚度' },
  { key: 'overall_score', label: '总分' },
] as const

const form = reactive<BlindScorePayload>({
  aroma_score: 8,
  acidity_score: 8,
  body_score: 8,
  overall_score: 8,
})
const submitting = ref(false)

watch(
  () => props.modelValue,
  open => {
    if (open) {
      form.aroma_score = 8
      form.acidity_score = 8
      form.body_score = 8
      form.overall_score = 8
    }
  },
)

async function submit() {
  submitting.value = true
  try {
    await submitBlindScore(props.sessionId, { ...form })
    ElMessage.success('评分已提交')
    emit('update:modelValue', false)
    emit('submitted')
  } finally {
    submitting.value = false
  }
}

function onVisibleChange(v: boolean) {
  emit('update:modelValue', v)
}
</script>

<style scoped>
.hint { margin-left: 12px; color: #999; font-size: 12px; }
</style>
