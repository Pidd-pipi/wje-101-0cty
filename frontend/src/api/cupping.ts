import request from '@/utils/request'
import type { UserInfo } from '@/constants/user'
import type { CoffeeBean } from '@/constants/bean'
import type { Cupping, CuppingListItem } from '@/constants/cupping'

export function listCuppings() {
  return request.get<never, CuppingListItem[]>('/cuppings')
}

export function getCupping(id: number | string) {
  return request.get<never, Cupping>(`/cuppings/${id}`)
}

export function createCupping(payload: { coffee_bean_id: number; participant_ids: number[] }) {
  return request.post<never, Cupping>('/cuppings', payload)
}

export function submitCuppingScore(
  id: number | string,
  payload: { aroma_score: number; acidity_score: number; body_score: number; overall_score: number },
) {
  return request.post<never, Cupping>(`/cuppings/${id}/submit`, payload)
}

export function revealCupping(id: number | string) {
  return request.post<never, Cupping>(`/cuppings/${id}/reveal`)
}

export function searchUsers(keyword: string) {
  return request.get<never, UserInfo[]>('/users/search', { params: { keyword } })
}

export { type CoffeeBean }
