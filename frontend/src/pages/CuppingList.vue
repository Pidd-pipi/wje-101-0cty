<template>
  <div class="page">
    <div class="head">
      <h1>杯测盲评</h1>
      <el-button type="primary" @click="openCreate">发起盲评</el-button>
    </div>
    <p class="tip">选择一款咖啡豆与三名参与者开评；每人仅可提交一次，全部提交前互相看不到评分，由发起人统一揭晓并计算各维平均分，偏差超过 {{ OUTLIER_THRESHOLD }} 分的评分标记为离群。</p>

    <el-row :gutter="16" v-loading="loading">
      <el-col v-for="item in items" :key="item.id" :xs="24" :sm="12" :md="8">
        <el-card class="cup-card" shadow="hover" @click="goDetail(item.id)">
          <div class="card-top">
            <el-tag :type="item.status === 'revealed' ? 'success' : 'warning'" size="small">
              {{ CuppingStatusMap[item.status] }}
            </el-tag>
            <span class="progress">{{ item.submitted_count }}/{{ item.participant_total }} 已提交</span>
          </div>
          <h3>{{ item.coffee_bean.name }}</h3>
          <div class="meta">{{ item.coffee_bean.origin || '-' }} · 发起人 {{ item.organizer.username }}</div>
          <div class="meta">{{ formatDateTime(item.created_at) }}</div>
        </el-card>
      </el-col>
    </el-row>
    <EmptyState v-if="!loading && !items.length" description="还没有盲评场次，发起一场吧" action-text="发起盲评" @action="openCreate" />

    <el-dialog v-model="showCreate" title="发起杯测盲评" width="560px">
      <el-form label-width="92px">
        <el-form-item label="咖啡豆" required>
          <el-select
            v-model="form.beanId"
            filterable
            remote
            reserve-keyword
            placeholder="搜索豆种名称"
            :remote-method="searchBeans"
            :loading="beanLoading"
            style="width: 100%"
          >
            <el-option v-for="b in beanOptions" :key="b.id" :label="`${b.name}（${b.origin || '未知产地'}）`" :value="b.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="`参与者（${PARTICIPANT_COUNT}人）`" required>
          <el-select
            v-model="form.participantIds"
            filterable
            remote
            reserve-keyword
            multiple
            collapse-tags
            :multiple-limit="PARTICIPANT_COUNT"
            placeholder="搜索用户名，选择三名不同用户"
            :remote-method="searchParticipants"
            :loading="userLoading"
            style="width: 100%"
          >
            <el-option
              v-for="u in userOptions"
              :key="u.id"
              :label="u.username"
              :value="u.id"
              :disabled="form.participantIds.length >= PARTICIPANT_COUNT && !form.participantIds.includes(u.id)"
            />
          </el-select>
        </el-form-item>
        <el-alert
          v-if="form.participantIds.length && form.participantIds.length !== PARTICIPANT_COUNT"
          type="info"
          :closable="false"
          :title="`还需选择 ${PARTICIPANT_COUNT - form.participantIds.length} 名参与者`"
        />
      </el-form>
      <template #footer>
        <el-button @click="showCreate = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="submitCreate">开评</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import EmptyState from '@/components/common/EmptyState.vue'
import { listCuppings, createCupping, searchUsers } from '@/api/cupping'
import { listBeans } from '@/api/bean'
import {
  CuppingStatusMap,
  OUTLIER_THRESHOLD,
  PARTICIPANT_COUNT,
  type CuppingListItem,
} from '@/constants/cupping'
import type { CoffeeBean } from '@/constants/bean'
import type { UserInfo } from '@/constants/user'
import { formatDateTime } from '@/utils/dateFormat'

const router = useRouter()
const items = ref<CuppingListItem[]>([])
const loading = ref(false)
const showCreate = ref(false)
const creating = ref(false)
const beanOptions = ref<CoffeeBean[]>([])
const userOptions = ref<UserInfo[]>([])
const beanLoading = ref(false)
const userLoading = ref(false)
const form = reactive<{ beanId: number | null; participantIds: number[] }>({ beanId: null, participantIds: [] })

onMounted(load)

async function load() {
  loading.value = true
  try {
    items.value = await listCuppings()
  } finally {
    loading.value = false
  }
}

function goDetail(id: number) {
  router.push(`/cupping/${id}`)
}

async function openCreate() {
  form.beanId = null
  form.participantIds = []
  showCreate.value = true
  await Promise.all([searchBeans(''), searchParticipants('')])
}

async function searchBeans(keyword: string) {
  beanLoading.value = true
  try {
    const res = await listBeans({ page: 1, page_size: 20, keyword })
    beanOptions.value = res.list
  } finally {
    beanLoading.value = false
  }
}

async function searchParticipants(keyword: string) {
  userLoading.value = true
  try {
    userOptions.value = await searchUsers(keyword)
  } finally {
    userLoading.value = false
  }
}

async function submitCreate() {
  if (!form.beanId) {
    ElMessage.warning('请选择一款咖啡豆')
    return
  }
  if (new Set(form.participantIds).size !== PARTICIPANT_COUNT || form.participantIds.length !== PARTICIPANT_COUNT) {
    ElMessage.warning(`必须选择 ${PARTICIPANT_COUNT} 名不同的参与者`)
    return
  }
  creating.value = true
  try {
    const cup = await createCupping({ coffee_bean_id: form.beanId, participant_ids: form.participantIds })
    ElMessage.success('盲评已发起')
    showCreate.value = false
    router.push(`/cupping/${cup.id}`)
  } finally {
    creating.value = false
  }
}
</script>

<style scoped>
.page { max-width: 1100px; margin: 0 auto; }
.head { display: flex; align-items: center; justify-content: space-between; }
.tip { color: #8a6a4f; background: #fff6ea; border-radius: 8px; padding: 10px 14px; font-size: 13px; }
.cup-card { margin-bottom: 16px; cursor: pointer; }
.card-top { display: flex; align-items: center; justify-content: space-between; }
.progress { color: #999; font-size: 12px; }
.meta { color: #999; font-size: 12px; margin-top: 6px; }
</style>
