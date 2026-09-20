export type CuppingStatus = 'collecting' | 'revealed'

export const CuppingStatusMap: Record<CuppingStatus, string> = {
  collecting: '收集中',
  revealed: '已揭晓',
}

// Deviation strictly greater than this value marks a score as outlier.
export const OUTLIER_THRESHOLD = 1.5
export const PARTICIPANT_COUNT = 3
export const MIN_SCORE = 0
export const MAX_SCORE = 10

export interface CuppingUser {
  id: number
  username: string
  avatar: string
}

export interface CuppingBean {
  id: number
  name: string
  origin: string
  process_method: string
}

export interface CuppingParticipant {
  user_id: number
  username: string
  avatar: string
  submitted: boolean
}

export interface CuppingScore {
  user_id: number
  username: string
  aroma_score: number
  acidity_score: number
  body_score: number
  overall_score: number
  outlier_aroma: boolean
  outlier_acidity: boolean
  outlier_body: boolean
  outlier_overall: boolean
}

export interface Cupping {
  id: number
  status: CuppingStatus
  organizer: CuppingUser
  coffee_bean: CuppingBean
  participants: CuppingParticipant[]
  submitted_count: number
  scores: CuppingScore[]
  avg_aroma: number
  avg_acidity: number
  avg_body: number
  avg_overall: number
  can_submit: boolean
  can_reveal: boolean
  revealed_at: string | null
  created_at: string
}

export interface CuppingListItem {
  id: number
  status: CuppingStatus
  organizer: CuppingUser
  coffee_bean: CuppingBean
  submitted_count: number
  participant_total: number
  created_at: string
  revealed_at: string | null
}
