import request from '@/utils/request'
import type { PageData } from '@/types/api'
import type { BlindScorePayload, BlindSession, BlindSessionBrief, UserBrief } from '@/constants/blind'

// 发起一场杯测盲评：一款咖啡豆 + 恰好三名参与者
export function createBlindTasting(payload: { coffee_bean_id: number; participant_ids: number[] }) {
  return request.post<never, BlindSession>('/blind-tastings', payload)
}

export function listBlindTastings(params: { page?: number; page_size?: number; scope?: 'all' | 'mine' }) {
  return request.get<never, PageData<BlindSessionBrief>>('/blind-tastings', { params })
}

export function getBlindTasting(id: number | string) {
  return request.get<never, BlindSession>(`/blind-tastings/${id}`)
}

// 参与者一次性提交四个维度的评分
export function submitBlindScore(id: number | string, payload: BlindScorePayload) {
  return request.post<never, BlindScorePayload>(`/blind-tastings/${id}/scores`, payload)
}

// 发起人在三人全部提交后统一揭晓
export function revealBlindTasting(id: number | string) {
  return request.post<never, BlindSession>(`/blind-tastings/${id}/reveal`)
}

export function searchBlindUsers(keyword: string) {
  return request.get<never, UserBrief[]>('/blind-tastings/user-search', { params: { keyword } })
}
