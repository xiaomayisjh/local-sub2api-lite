import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ImportDataModal from '@/components/admin/account/ImportDataModal.vue'
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
    accounts: {
      importData: vi.fn()
    }
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

describe('ImportDataModal', () => {
  beforeEach(() => {
    showError.mockReset()
    showSuccess.mockReset()
    vi.mocked(adminAPI.accounts.importData).mockReset()
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
    expect(showError).toHaveBeenCalledWith('admin.accounts.dataImportSelectFile')
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

    expect(showError).toHaveBeenCalledWith('admin.accounts.dataImportParseFailed')
  })

  it('imports accounts with default group binding enabled', async () => {
    const importData = vi.mocked(adminAPI.accounts.importData)
    importData.mockResolvedValue({
      proxy_created: 0,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 1,
      account_failed: 0
    })

    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })

    const payload = {
      type: 'sub2api-data',
      version: 1,
      exported_at: '2026-05-25T00:00:00Z',
      proxies: [],
      accounts: []
    }
    const input = wrapper.find('input[type="file"]')
    const file = new File([JSON.stringify(payload)], 'data.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', {
      value: () => Promise.resolve(JSON.stringify(payload))
    })
    Object.defineProperty(input.element, 'files', {
      value: [file]
    })

    await input.trigger('change')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(importData).toHaveBeenCalledWith({
      data: payload,
      skip_default_group_bind: false
    })
    expect(showSuccess).toHaveBeenCalled()
  })

  it('imports multiple JSON files and aggregates partial failures', async () => {
    const importData = vi.mocked(adminAPI.accounts.importData)
    importData
      .mockResolvedValueOnce({
        proxy_created: 1,
        proxy_reused: 0,
        proxy_failed: 0,
        account_created: 1,
        account_failed: 0
      })
      .mockResolvedValueOnce({
        proxy_created: 0,
        proxy_reused: 1,
        proxy_failed: 1,
        account_created: 0,
        account_failed: 1,
        errors: [
          {
            kind: 'account',
            name: 'bad-account',
            message: 'duplicate'
          }
        ]
      })

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
      proxies: [],
      accounts: [{ name: 'a' }]
    }
    const payloadB = {
      type: 'sub2api-data',
      version: 1,
      exported_at: '2026-05-25T00:00:00Z',
      proxies: [],
      accounts: [{ name: 'b' }]
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
    expect(showError).toHaveBeenCalledWith('admin.accounts.dataImportCompletedWithErrors')
    expect(wrapper.emitted('imported')?.[0]).toEqual([{ close: false }])
    expect(wrapper.text()).toContain('b.json: duplicate')
  })
})
