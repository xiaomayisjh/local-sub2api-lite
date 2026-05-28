import { apiClient } from '../client'

export interface LocalInfo {
  data_dir: string
  default_api_key: string
  generated_admin_password?: string
  server_host: string
  server_port: number
  run_mode: string
  config_path?: string
}

export interface LocalPortCheck {
  port: number
  available: boolean
  current_port: number
  host: string
  message?: string
}

export interface LocalPortUpdateResult {
  port: number
  need_restart: boolean
  config_path: string
  message: string
}

export async function getLocalInfo(): Promise<LocalInfo> {
  const { data } = await apiClient.get<LocalInfo>('/admin/local/info')
  return data
}

export async function checkLocalPort(port: number): Promise<LocalPortCheck> {
  const { data } = await apiClient.get<LocalPortCheck>('/admin/local/port/check', {
    params: { port }
  })
  return data
}

export async function updateLocalPort(port: number): Promise<LocalPortUpdateResult> {
  const { data } = await apiClient.put<LocalPortUpdateResult>('/admin/local/port', { port })
  return data
}
