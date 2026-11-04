/**
 * 公开参数读取：登录页等未登录场景可读取站点名称与 Logo。
 */
import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from './client'
import type { Parameter } from './types'

export interface SiteConfig {
  name: string
  logo: string
}

/** 读取 public=true 的系统参数，失败时回退到内置品牌信息。 */
export function useSiteConfig(): SiteConfig {
  const query = useQuery({
    queryKey: ['public-parameters'],
    queryFn: () => api.get<Parameter[]>('/parameters/public'),
    staleTime: 5 * 60_000,
    retry: false,
  })
  const value = (key: string) => query.data?.find((item) => item.key === key)?.value?.trim() ?? ''
  const name = value('site.name') || 'x-admin'
  const logo = value('site.logo')

  // 让浏览器标题也跟随站点名称。
  useEffect(() => {
    document.title = name
  }, [name])

  return { name, logo }
}
