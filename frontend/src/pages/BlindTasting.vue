<template>
  <div class="page">
    <template v-if="!detailId">
      <div class="list-head">
        <h1>杯测盲评</h1>
        <div class="head-actions">
          <el-radio-group v-model="scope" @change="load">
            <el-radio-button label="all">全部</el-radio-button>
            <el-radio-button label="mine">与我相关</el-radio-button>
          </el-radio-group>
          <el-button v-if="isLoggedIn" type="primary" @click="showCreate = true">发起盲评</el-button>
          <el-button v-else type="primary" @click="router.push('/login')">登录后发起</el-button>
        </div>
      </div>
      <el-table :data="store.sessions" stripe style="width: 100%">
        <el-table-column label="咖啡豆" min-width="180">
          <template #default="{ row }">{{ row.coffee_bean_name }}</template>
        </el-table-column>
        <el-table-column label="发起人" width="120">
          <template #default="{ row }">{{ row.host_name }}</template>
        </el-table-column>
        <el-table-column label="提交进度" width="110">
          <template #default="{ row }">
            <el-tag size="small" :type="row.submitted_count === row.participant_count ? 'success' : 'warning'">
              {{ row.submitted_count }}/{{ row.participant_count }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'revealed' ? 'success' : 'info'">{{ row.status_text }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="开评时间" width="160">
          <template #default="{ row }">{{ row.created_at }}</template>
        </el-table-column>
        <el-table-column label="操作" width="110">
          <template #default="{ row }">
            <el-button size="small" type="primary" link @click="openDetail(row.id)">查看</el-button>
          </template>
        </el-table-column>
      </el-table>
      <EmptyState v-if="!store.sessions.length" description="暂无盲评，发起一场吧" />
      <el-pagination
        v-if="store.total > pageSize"
        class="pager"
        layout="prev, pager, next"
        :total="store.total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="onPage"
      />
    </template>

    <template v-else>
      <el-page-header content="盲评详情" @back="backToList" class="detail-header" />
      <el-skeleton v-if="!session" :rows="6" animated />
      <template v-else>
        <el-card class="detail-card" shadow="never">
          <div class="detail-top">
            <div>
              <h2>{{ session.coffee_bean_name }}</h2>
              <div class="meta">
                发起人：{{ session.host_name }}
                <el-divider direction="vertical" />
                开评：{{ session.created_at }}
                <template v-if="session.revealed_at">
                  <el-divider direction="vertical" />揭晓：{{ session.revealed_at }}
                </template>
              </div>
            </div>
            <el-tag :type="session.status === 'revealed' ? 'success' : 'warning'" size="large">
              {{ session.status_text }}
            </el-tag>
          </div>
          <div class="progress">
            提交进度：
            <el-tag size="small" :type="allSubmitted ? 'success' : 'warning'">
              {{ session.submitted_count }}/{{ session.participant_ids.length }}
            </el-tag>
            <span v-if="session.status === 'ongoing' && !allSubmitted" class="mask-hint">全部提交前，任何人都看不到他人评分</span>
          </div>
          <div class="actions">
            <el-button
              v-if="session.is_participant && session.status === 'ongoing' && !session.my_submitted"
              type="primary"
              @click="showScore = true"
            >提交我的评分</el-button>
            <el-tag v-else-if="session.is_participant && session.my_submitted && session.status === 'ongoing'" type="info">
              你已提交，等待其他人
            </el-tag>
            <el-button
              v-if="session.is_host && session.status === 'ongoing'"
              type="success"
              :disabled="!allSubmitted"
              :loading="revealing"
              @click="onReveal"
            >{{ allSubmitted ? '统一揭晓' : '等待全部提交后揭晓' }}</el-button>
            <el-button @click="refreshDetail">刷新回读</el-button>
          </div>
        </el-card>

        <el-card class="score-card" shadow="never">
          <template #header>
            <span v-if="session.status === 'revealed'">评分明细（含各维平均分，离群已标注）</span>
            <span v-else>匿名评分板（仅显示自己的分数）</span>
          </template>
          <el-table :data="session.scores" border>
            <el-table-column label="参评者" width="180">
              <template #default="{ row, $index }">
                <span v-if="session.status === 'ongoing'">参评者 {{ $index + 1 }}</span>
                <span v-else>
                  <UserAvatar :name="row.username" :size="22" />
                  {{ row.username }}
                </span>
                <el-tag v-if="isMe(row.user_id)" size="small" type="primary" effect="plain" class="me-tag">我</el-tag>
              </template>
            </el-table-column>
            <el-table-column v-for="dim in DIMENSIONS" :key="dim.key" :label="dim.label" width="150">
              <template #default="{ row }">
                <template v-if="row.submitted && (session.status === 'revealed' || isMe(row.user_id))">
                  <span :class="{ outlier: row[dim.outlierKey] }">{{ Number(row[dim.key]).toFixed(1) }}</span>
                  <el-tooltip v-if="row[dim.outlierKey]" content="离群：与该维平均分偏差超过 1.5" placement="top">
                    <el-tag size="small" type="danger" effect="plain" class="outlier-tag">离群</el-tag>
                  </el-tooltip>
                </template>
                <span v-else-if="row.submitted" class="masked">已提交（匿名）</span>
                <span v-else class="pending">未提交</span>
              </template>
            </el-table-column>
          </el-table>

          <div v-if="session.averages" class="averages">
            <span class="avg-title">各维平均分：</span>
            <el-tag v-for="dim in DIMENSIONS" :key="dim.key" type="success" effect="light" size="large" class="avg-tag">
              {{ dim.label }} {{ session.averages[dim.avgKey].toFixed(2) }}
            </el-tag>
            <span class="avg-rule">偏差 &gt; {{ BLIND_OUTLIER_THRESHOLD }} 分标记为离群</span>
          </div>
        </el-card>
      </template>
    </template>

    <BlindCreateDialog v-if="isLoggedIn" v-model="showCreate" @created="onCreated" />
    <BlindScoreDialog
      v-if="session"
      v-model="showScore"
      :session-id="session.id"
      @submitted="refreshDetail"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import EmptyState from '@/components/common/EmptyState.vue'
import UserAvatar from '@/components/common/UserAvatar.vue'
import BlindCreateDialog from '@/components/blind/BlindCreateDialog.vue'
import BlindScoreDialog from '@/components/blind/BlindScoreDialog.vue'
import { useBlindTastingStore } from '@/stores/useBlindTastingStore'
import { useAuth } from '@/hooks/useAuth'
import { revealBlindTasting } from '@/api/blind'
import { BLIND_OUTLIER_THRESHOLD } from '@/constants/blind'

const route = useRoute()
const router = useRouter()
const store = useBlindTastingStore()
const { isLoggedIn, user } = useAuth()

const page = ref(1)
const pageSize = 10
const scope = ref<'all' | 'mine'>('all')
const showCreate = ref(false)
const showScore = ref(false)
const revealing = ref(false)

const detailId = computed(() => (route.name === 'blindDetail' ? (route.params.id as string) : ''))
const session = computed(() => store.current)
const allSubmitted = computed(() => !!session.value && session.value.submitted_count === session.value.participant_ids.length)

const DIMENSIONS = [
  { key: 'aroma_score', avgKey: 'aroma', outlierKey: 'aroma_outlier', label: '香气' },
  { key: 'acidity_score', avgKey: 'acidity', outlierKey: 'acidity_outlier', label: '酸质' },
  { key: 'body_score', avgKey: 'body', outlierKey: 'body_outlier', label: '醇厚度' },
  { key: 'overall_score', avgKey: 'overall', outlierKey: 'overall_outlier', label: '总分' },
] as const

onMounted(() => {
  if (detailId.value) void store.refresh(detailId.value)
  else void load()
})

watch(detailId, id => {
  if (id) void store.refresh(id)
  else void load()
})

async function load() {
  await store.load({ page: page.value, page_size: pageSize, scope: scope.value })
}

function onPage(p: number) {
  page.value = p
  void load()
}

function openDetail(id: number) {
  router.push(`/blind-tastings/${id}`)
}

function backToList() {
  store.clearCurrent()
  router.push('/blind-tastings')
}

function onCreated(id: number) {
  router.push(`/blind-tastings/${id}`)
}

async function refreshDetail() {
  if (detailId.value) await store.refresh(detailId.value)
}

async function onReveal() {
  if (!session.value) return
  try {
    await ElMessageBox.confirm('揭晓后将公开全部评分并计算各维平均分，且不可重复揭晓。确认揭晓？', '统一揭晓', {
      type: 'warning',
      confirmButtonText: '确认揭晓',
      cancelButtonText: '再等等',
    })
  } catch {
    return
  }
  revealing.value = true
  try {
    const revealed = await revealBlindTasting(session.value.id)
    store.current = revealed
    ElMessage.success('盲评结果已揭晓')
  } finally {
    revealing.value = false
  }
}

function isMe(uid: number) {
  return user.value?.id === uid
}
</script>

<style scoped>
.page { max-width: 1080px; margin: 0 auto; }
.list-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.head-actions { display: flex; gap: 12px; align-items: center; }
.pager { margin-top: 16px; justify-content: center; }
.detail-header { margin-bottom: 16px; }
.detail-card, .score-card { margin-bottom: 16px; }
.detail-top { display: flex; justify-content: space-between; align-items: flex-start; }
.meta { color: #888; font-size: 13px; margin-top: 6px; }
.progress { margin: 14px 0; color: #555; }
.mask-hint { margin-left: 10px; color: #b8823a; font-size: 13px; }
.actions { display: flex; gap: 12px; align-items: center; }
.me-tag { margin-left: 6px; }
.masked { color: #b8823a; }
.pending { color: #bbb; }
.outlier { color: #e5534b; font-weight: 700; }
.outlier-tag { margin-left: 6px; }
.averages { margin-top: 16px; display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.avg-title { font-weight: 700; color: #7b4b2a; }
.avg-rule { color: #999; font-size: 12px; }
</style>
