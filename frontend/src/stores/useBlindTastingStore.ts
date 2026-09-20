import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getBlindTasting, listBlindTastings } from '@/api/blind'
import type { BlindSession, BlindSessionBrief } from '@/constants/blind'

export const useBlindTastingStore = defineStore('blindTasting', () => {
  const sessions = ref<BlindSessionBrief[]>([])
  const total = ref(0)
  const current = ref<BlindSession | null>(null)

  async function load(params: { page?: number; page_size?: number; scope?: 'all' | 'mine' } = {}) {
    const res = await listBlindTastings(params)
    sessions.value = res.list
    total.value = res.total
  }

  async function refresh(id: number | string) {
    current.value = await getBlindTasting(id)
    return current.value
  }

  function clearCurrent() {
    current.value = null
  }

  return { sessions, total, current, load, refresh, clearCurrent }
})
