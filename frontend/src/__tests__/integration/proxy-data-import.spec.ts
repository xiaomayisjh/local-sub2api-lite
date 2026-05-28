import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ImportDataModal from '@/components/admin/proxy/ImportDataModal.vue'
import { adminAPI } from '@/api/admin'

const showError = vi.fn()
const showSuccess = vi.fn()

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    proxies: {
      importData: vi.fn()
    }
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

describe('Proxy ImportDataModal', () => {
  beforeEach(() => {
    showError.mockReset()
    showSuccess.mockReset()
    vi.mocked(adminAPI.proxies.importData).mockReset()
  })

  it('未选择文件时提示错误', async () => {
    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })

    await wrapper.find('form').trigger('submit')
    expect(showError).toHaveBeenCalledWith('admin.proxies.dataImportSelectFile')
  })

  it('无效 JSON 时提示解析失败', async () => {
    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })

    const input = wrapper.find('input[type="file"]')
    const file = new File(['invalid json'], 'data.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', {
      value: () => Promise.resolve('invalid json')
    })
    Object.defineProperty(input.element, 'files', {
      value: [file]
    })

    await input.trigger('change')
    await wrapper.find('form').trigger('submit')
    await Promise.resolve()

    expect(showError).toHaveBeenCalledWith('admin.proxies.dataImportParseFailed')
  })

  it('imports multiple proxy JSON files and keeps successful imports', async () => {
    const importData = vi.mocked(adminAPI.proxies.importData)
    importData
      .mockResolvedValueOnce({
        proxy_created: 1,
        proxy_reused: 0,
        proxy_failed: 0,
        account_created: 0,
        account_failed: 0
      })
      .mockRejectedValueOnce(new Error('upstream failed'))

    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })

    const payloadA = {
      type: 'sub2api-data',
      version: 1,
      exported_at: '2026-05-25T00:00:00Z',
      proxies: [{ key: 'a' }]
    }
    const payloadB = {
      type: 'sub2api-data',
      version: 1,
      exported_at: '2026-05-25T00:00:00Z',
      proxies: [{ key: 'b' }]
    }
    const fileA = new File([JSON.stringify(payloadA)], 'a.json', { type: 'application/json' })
    const fileB = new File([JSON.stringify(payloadB)], 'b.json', { type: 'application/json' })
    Object.defineProperty(fileA, 'text', {
      value: () => Promise.resolve(JSON.stringify(payloadA))
    })
    Object.defineProperty(fileB, 'text', {
      value: () => Promise.resolve(JSON.stringify(payloadB))
    })

    const input = wrapper.find('input[type="file"]')
    Object.defineProperty(input.element, 'files', {
      value: [fileA, fileB]
    })

    await input.trigger('change')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(importData).toHaveBeenCalledTimes(2)
    expect(showError).toHaveBeenCalledWith('admin.proxies.dataImportCompletedWithErrors')
    expect(wrapper.emitted('imported')?.[0]).toEqual([{ close: false }])
    expect(wrapper.text()).toContain('upstream failed')
  })
})
