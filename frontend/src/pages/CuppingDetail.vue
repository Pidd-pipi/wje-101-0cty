<template>
  <div class="page" v-loading="loading">
    <template v-if="cup">
      <el-page-header @back="$router.push('/cupping')" content="盲评详情" />
      <el-card class="block">
        <div class="title-row">
          <div>
            <h1>{{ cup.coffee_bean.name }}</h1>
            <div class="meta">{{ cup.coffee_bean.origin || '-' }} · 发起人 {{ cup.organizer.username }} · {{ formatDateTime(cup.created_at) }}</div>
          </div>
          <el-tag :type="cup.status === 'revealed' ? 'success' : 'warning'" size="large">
            {{ CuppingStatusMap[cup.status] }}
          </el-tag>
        </div>

        <el-divider />
        <h3>参与者（{{ cup.submitted_count }}/{{ cup.participants.length }} 已提交）</h3>
        <div class="participants">
          <el-tag
            v-for="p in cup.participants"
            :key="p.user_id"
            :type="p.submitted ? 'success' : 'info'"
            class="part-tag"
          >
            {{ cup.status === 'revealed' ? p.username : `参与者 #${p.user_id}` }}
            {{ p.submitted ? ' · 已提交' : ' · 待提交' }}
          </el-tag>
        </div>
        <el-alert
          v-if="cup.status === 'collecting'"
          class="sealed-tip"
          type="warning"
          :closable="false"
          title="盲评进行中：全部提交前看不到任何他人评分。"
        />

        <!-- 揭晓结果 -->
        <template v-if="cup.status === 'revealed'">
          <el-divider />
          <h3>各维平均分</h3>
          <el-descriptions :column="4" border>
            <el-descriptions-item label="香气">{{ cup.avg_aroma.toFixed(1) }}</el-descriptions-item>
            <el-descriptions-item label="酸质">{{ cup.avg_acidity.toFixed(1) }}</el-descriptions-item>
            <el-descriptions-item label="醇厚度">{{ cup.avg_body.toFixed(1) }}</el-descriptions-item>
            <el-descriptions-item label="总分">{{ cup.avg_overall.toFixed(1) }}</el-descriptions-item>
          </el-descriptions>

          <h3 class="score-head">评分明细（离群：偏差 &gt; {{ OUTLIER_THRESHOLD }} 分）</h3>
          <el-table :data="cup.scores" border stripe>
            <el-table-column label="评分人" prop="username" width="120" />
            <el-table-column label="香气">
              <template #default="{ row }">
                <span :class="{ outlier: row.outlier_aroma }">{{ row.aroma_score.toFixed(1) }}</span>
                <el-tag v-if="row.outlier_aroma" type="danger" size="small" class="out-tag">离群</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="酸质">
              <template #default="{ row }">
                <span :class="{ outlier: row.outlier_acidity }">{{ row.acidity_score.toFixed(1) }}</span>
                <el-tag v-if="row.outlier_acidity" type="danger" size="small" class="out-tag">离群</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="醇厚度">
              <template #default="{ row }">
                <span :class="{ outlier: row.outlier_body }">{{ row.body_score.toFixed(1) }}</span>
                <el-tag v-if="row.outlier_body" type="danger" size="small" class="out-tag">离群</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="总分">
              <template #default="{ row }">
                <span :class="{ outlier: row.outlier_overall }">{{ row.overall_score.toFixed(1) }}</span>
                <el-tag v-if="row.outlier_overall" type="danger" size="small" class="out-tag">离群</el-tag>
              </template>
            </el-table-column>
          </el-table>
        </template>

        <!-- 我的评分提交（仅参与者、未提交、未揭晓） -->
        <template v-if="cup.can_submit">
          <el-divider />
          <h3>提交我的盲评（仅一次，提交后不可修改）</h3>
          <el-form label-width="90px" class="score-form">
            <el-form-item label="香气">
              <el-rate v-model="form.aroma_score" :max="10" allow-half show-score score-template="{value}" />
            </el-form-item>
            <el-form-item label="酸质">
              <el-rate v-model="form.acidity_score" :max="10" allow-half show-score score-template="{value}" />
            </el-form-item>
            <el-form-item label="醇厚度">
              <el-rate v-model="form.body_score" :max="10" allow-half show-score score-template="{value}" />
            </el-form-item>
            <el-form-item label="总分">
              <el-rate v-model="form.overall_score" :max="10" allow-half show-score score-template="{value}" />
            </el-form-item>
          </el-form>
          <el-button type="primary" :loading="submitting" @click="submit">提交评分</el-button>
        </template>

        <!-- 发起人揭晓 -->
        <div v-if="cup.can_reveal" class="reveal-row">
          <el-divider />
          <el-alert
            v-if="cup.submitted_count < cup.participants.length"
            type="info"
            :closable="false"
            :title="`还有 ${cup.participants.length - cup.submitted_count} 人未提交，暂不能揭晓`"
            class="reveal-tip"
          />
          <el-button type="success" :loading="revealing" :disabled="cup.submitted_count < cup.participants.length" @click="reveal">
            统一揭晓
          </el-button>
        </div>

        <el-alert
          v-if="!cup.can_submit && cup.status === 'collecting' && !isOrganizer"
          type="info"
          :closable="false"
          title="你已完成提交，等待其他人评分与发起人揭晓。"
          class="block"
        />
      </el-card>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getCupping, submitCuppingScore, revealCupping } from '@/api/cupping'
import { CuppingStatusMap, OUTLIER_THRESHOLD, type Cupping } from '@/constants/cupping'
import { useAuth } from '@/hooks/useAuth'
import { formatDateTime } from '@/utils/dateFormat'

const route = useRoute()
const { user } = useAuth()
const cup = ref<Cupping | null>(null)
const loading = ref(false)
const submitting = ref(false)
const revealing = ref(false)
const form = reactive({ aroma_score: 0, acidity_score: 0, body_score: 0, overall_score: 0 })

const isOrganizer = computed(() => !!user.value && cup.value?.organizer.id === user.value.id)

onMounted(refresh)

async function refresh() {
  loading.value = true
  try {
    cup.value = await getCupping(route.params.id as string)
  } finally {
    loading.value = false
  }
}

async function submit() {
  if ([form.aroma_score, form.acidity_score, form.body_score, form.overall_score].some((v) => v <= 0)) {
    ElMessage.warning('请为香气、酸质、醇厚度和总分都打分')
    return
  }
  try {
    await ElMessageBox.confirm('盲评评分仅可提交一次，确认提交？', '确认提交', { type: 'warning' })
  } catch {
    return
  }
  submitting.value = true
  try {
    cup.value = await submitCuppingScore(route.params.id as string, { ...form })
    ElMessage.success('评分已提交')
  } finally {
    submitting.value = false
  }
}

async function reveal() {
  try {
    await ElMessageBox.confirm('揭晓后将公开全部评分并计算平均分，确认揭晓？', '确认揭晓', { type: 'warning' })
  } catch {
    return
  }
  revealing.value = true
  try {
    cup.value = await revealCupping(route.params.id as string)
    ElMessage.success('盲评已揭晓')
  } finally {
    revealing.value = false
  }
}
</script>

<style scoped>
.page { max-width: 1000px; margin: 0 auto; }
.block { margin-top: 12px; }
.title-row { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.meta { color: #999; font-size: 13px; margin-top: 6px; }
.participants { display: flex; flex-wrap: wrap; gap: 10px; }
.part-tag { font-size: 13px; }
.sealed-tip { margin-top: 14px; }
.score-head { margin-top: 18px; }
.score-form { max-width: 420px; }
.reveal-row { margin-top: 8px; }
.reveal-tip { margin-bottom: 12px; }
.outlier { color: #e64340; font-weight: 700; }
.out-tag { margin-left: 6px; }
</style>
