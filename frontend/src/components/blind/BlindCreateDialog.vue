<template>
  <el-dialog :model-value="modelValue" title="发起杯测盲评" width="520px" @update:model-value="onVisibleChange" @open="onOpen">
    <el-form label-width="92px">
      <el-form-item label="咖啡豆" required>
        <el-select
          v-model="form.coffee_bean_id"
          filterable
          clearable
          placeholder="选择一款咖啡豆"
          style="width: 360px"
        >
          <el-option v-for="b in beans" :key="b.id" :label="`${b.name}（${b.origin || '未知产地'}）`" :value="b.id" />
        </el-select>
      </el-form-item>
      <el-form-item v-for="n in BLIND_PARTICIPANT_COUNT" :key="n" :label="`参与者 ${n}`" required>
        <el-select
          v-model="form.participant_ids[n - 1]"
          filterable
          remote
          clearable
          reserve-keyword
          :remote-method="(query: string) => searchUsers(n - 1, query)"
          :loading="loading"
          placeholder="搜索用户名，选择一名参评者"
          style="width: 360px"
        >
          <el-option
            v-for="u in userOptions[n - 1]"
            :key="u.id"
            :label="u.username"
            :value="u.id"
            :disabled="isPickedElsewhere(n - 1, u.id)"
          />
        </el-select>
      </el-form-item>
      <el-alert type="info" :closable="false" show-icon>
        <template #title>开评后每人只能提交一次香气、酸质、醇厚度、总分；全部提交前互不可见，由你统一揭晓。</template>
      </el-alert>
    </el-form>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submit">开评</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { listBeans } from '@/api/bean'
import { createBlindTasting, searchBlindUsers } from '@/api/blind'
import type { CoffeeBean } from '@/constants/bean'
import type { UserBrief } from '@/constants/blind'
import { BLIND_PARTICIPANT_COUNT } from '@/constants/blind'

defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void
  (e: 'created', id: number): void
}>()

const beans = ref<CoffeeBean[]>([])
const userOptions = ref<UserBrief[][]>([[], [], []])
const loading = ref(false)
const submitting = ref(false)
const form = reactive<{ coffee_bean_id: number | undefined; participant_ids: (number | undefined)[] }>({
  coffee_bean_id: undefined,
  participant_ids: [undefined, undefined, undefined],
})

async function onOpen() {
  form.coffee_bean_id = undefined
  form.participant_ids = [undefined, undefined, undefined]
  if (!beans.value.length) {
    const res = await listBeans({ page: 1, page_size: 100 })
    beans.value = res.list
  }
  await Promise.all([0, 1, 2].map(i => searchUsers(i, '')))
}

function onVisibleChange(v: boolean) {
  emit('update:modelValue', v)
}

async function searchUsers(slot: number, keyword: string) {
  loading.value = true
  try {
    userOptions.value[slot] = await searchBlindUsers(keyword)
  } finally {
    loading.value = false
  }
}

function isPickedElsewhere(slot: number, id: number) {
  return form.participant_ids.some((pid, i) => i !== slot && pid === id)
}

async function submit() {
  if (!form.coffee_bean_id) {
    ElMessage.warning('请选择一款咖啡豆')
    return
  }
  const ids = form.participant_ids
  if (ids.some(id => !id)) {
    ElMessage.warning(`必须选择恰好 ${BLIND_PARTICIPANT_COUNT} 名参与者`)
    return
  }
  if (new Set(ids).size !== BLIND_PARTICIPANT_COUNT) {
    ElMessage.warning('参与者不能重复')
    return
  }
  submitting.value = true
  try {
    const session = await createBlindTasting({
      coffee_bean_id: form.coffee_bean_id as number,
      participant_ids: ids as number[],
    })
    ElMessage.success('盲评已开评')
    emit('update:modelValue', false)
    emit('created', session.id)
  } finally {
    submitting.value = false
  }
}
</script>
