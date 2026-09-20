export type BlindStatus = 'ongoing' | 'revealed'

export const BlindStatusMap: Record<BlindStatus, string> = {
  ongoing: '开评中',
  revealed: '已揭晓',
}

// 每场盲评固定三名参与者
export const BLIND_PARTICIPANT_COUNT = 3
// 与维平均分偏差超过该值（严格大于）标记为离群
export const BLIND_OUTLIER_THRESHOLD = 1.5
export const BLIND_MIN_SCORE = 0
export const BLIND_MAX_SCORE = 10

export interface BlindScoreView {
  user_id: number
  username: string
  avatar: string
  submitted: boolean
  aroma_score?: number
  acidity_score?: number
  body_score?: number
  overall_score?: number
  aroma_outlier?: boolean
  acidity_outlier?: boolean
  body_outlier?: boolean
  overall_outlier?: boolean
}

export interface BlindAverages {
  aroma: number
  acidity: number
  body: number
  overall: number
}

export interface BlindSession {
  id: number
  host_id: number
  host_name: string
  coffee_bean_id: number
  coffee_bean_name: string
  status: BlindStatus
  status_text: string
  revealed_at?: string
  created_at: string
  participant_ids: number[]
  submitted_count: number
  is_host: boolean
  is_participant: boolean
  my_submitted: boolean
  scores: BlindScoreView[]
  averages?: BlindAverages
}

export interface BlindSessionBrief {
  id: number
  host_id: number
  host_name: string
  coffee_bean_id: number
  coffee_bean_name: string
  status: BlindStatus
  status_text: string
  submitted_count: number
  participant_count: number
  created_at: string
}

export interface BlindScorePayload {
  aroma_score: number
  acidity_score: number
  body_score: number
  overall_score: number
}

export interface UserBrief {
  id: number
  username: string
  avatar: string
}
