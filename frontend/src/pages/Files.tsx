/**
 * 文件管理：上传、筛选、下载与删除。
 */
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App, Button, Drawer, Input, Select, Upload } from 'antd'
import dayjs from 'dayjs'
import { api, saveBlob } from '../api/client'
import type { FileItem, FileShare, PageResult } from '../api/types'
import { useAuth } from '../stores/auth'
import { PageHeader } from '../components/PageHeader'
import { DataTable } from '../components/DataTable'
import { Icon } from '../components/Icon'
import { formatSize } from '../utils/format'
import { FILE_KIND_OPTIONS, fileKind, fileKindIcon, type FileKind } from '../utils/file'

export function Files() {
  const { message, modal } = App.useApp()
  const queryClient = useQueryClient()
  const can = useAuth((state) => state.can)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [keywordDraft, setKeywordDraft] = useState('')
  const [kind, setKind] = useState<FileKind | 'all'>('all')
  const [shareFile, setShareFile] = useState<FileItem>()
  const [latestShareURL, setLatestShareURL] = useState('')

  // 文件列表按类型、关键字和分页查询。
  const query = useQuery({
    queryKey: ['files', page, pageSize, keyword, kind],
    queryFn: () =>
      api.get<PageResult<FileItem>>('/files', {
        page,
        page_size: pageSize,
        keyword,
        kind: kind === 'all' ? undefined : kind,
      }),
  })

  const invalidate = () => void queryClient.invalidateQueries({ queryKey: ['files'] })

  // 上传和删除都会让列表失效，统一在成功后刷新。
  const uploadMutation = useMutation({
    mutationFn: (file: File) => api.upload<FileItem>('/files', file),
    onSuccess: (file) => {
      message.success(`${file.name} 已上传`)
      invalidate()
    },
    onError: (error: Error) => message.error(error.message),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.del(`/files/${id}`),
    onSuccess: () => {
      message.success('文件已删除')
      invalidate()
    },
    onError: (error: Error) => message.error(error.message),
  })

  const sharesQuery = useQuery({
    queryKey: ['file-shares', shareFile?.id],
    queryFn: () => api.get<FileShare[]>(`/files/${shareFile?.id}/shares`),
    enabled: !!shareFile,
  })

  const createShareMutation = useMutation({
    mutationFn: (id: number) =>
      api.post<{ id: number; url: string; expires_at: string }>(`/files/${id}/shares`, { expires: 86400 }),
    onSuccess: async (result) => {
      const absolute = new URL(result.url, window.location.origin).toString()
      setLatestShareURL(absolute)
      void queryClient.invalidateQueries({ queryKey: ['file-shares'] })
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(absolute)
        message.success('分享链接已复制，24 小时内有效')
      }
    },
    onError: (error: Error) => message.error(error.message),
  })

  const revokeShareMutation = useMutation({
    mutationFn: ({ fileID, shareID }: { fileID: number; shareID: number }) =>
      api.del(`/files/${fileID}/shares/${shareID}`),
    onSuccess: () => {
      message.success('分享链接已撤销')
      setLatestShareURL('')
      void queryClient.invalidateQueries({ queryKey: ['file-shares'] })
    },
    onError: (error: Error) => message.error(error.message),
  })

  const copyLatestShare = async () => {
    if (!latestShareURL) return
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(latestShareURL)
      message.success('分享链接已复制')
    } else {
      window.prompt('复制分享链接', latestShareURL)
    }
  }

  // 下载走后端鉴权接口，再用 Blob 触发浏览器保存。
  const download = async (file: FileItem) => {
    try {
      saveBlob(await api.download(`/files/${file.id}/download`), file.name)
    } catch (error) {
      message.error(error instanceof Error ? error.message : '下载失败')
    }
  }

  return (
    <>
      <PageHeader
        title="文件管理"
        sub={`${query.data?.total ?? 0} 个文件 · 本地磁盘 data/uploads · 存储层可替换`}
        actions={
          can('system:file:upload') ? (
            <Upload
              showUploadList={false}
              beforeUpload={(file) => {
                uploadMutation.mutate(file)
                return false
              }}
            >
              <button className="btn primary" type="button">
                <Icon name="upload" size={13} /> 上传文件
              </button>
            </Upload>
          ) : undefined
        }
      />

      <DataTable<FileItem>
        storageKey="files"
        rowKey="id"
        data={query.data?.list ?? []}
        loading={query.isFetching}
        total={query.data?.total ?? 0}
        page={page}
        pageSize={pageSize}
        onPageChange={(nextPage, nextSize) => { setPage(nextPage); setPageSize(nextSize) }}
        onRefresh={() => void query.refetch()}
        columns={[
          {
            title: '文件',
            key: 'name',
            width: 260,
            render: (_, row) => {
              const rowKind = fileKind(row)
              return (
                <div className="file-name">
                  <span className={`file-icon ${rowKind}`}>
                    {rowKind === 'image' && row.url ? (
                      <img src={row.url} alt="" loading="lazy" />
                    ) : (
                      <Icon name={fileKindIcon(row)} size={13} />
                    )}
                  </span>
                  <span>
                    <b style={{ display: 'block', fontSize: 11 }}>{row.name}</b>
                    <span className="mono">{row.mime || row.ext}</span>
                  </span>
                </div>
              )
            },
          },
          { title: '大小', dataIndex: 'size', key: 'size', width: 100, render: (value: number) => <span className="mono">{formatSize(value)}</span> },
          { title: '上传时间', dataIndex: 'created_at', key: 'created_at', width: 150, render: (value: string) => <span className="mono">{dayjs(value).format('MM-DD HH:mm')}</span> },
          {
            title: '操作',
            key: 'actions',
            width: 200,
            fixed: 'right',
            render: (_, row) => (
              <div className="inline-actions">
                {can('system:file:download') && <Button size="small" type="text" onClick={() => void download(row)}>下载</Button>}
                {can('system:file:download') && <Button size="small" type="text" onClick={() => { setShareFile(row); setLatestShareURL('') }}>分享</Button>}
                {can('system:file:delete') && (
                  <Button
                    size="small"
                    type="text"
                    danger
                    onClick={() => {
                      modal.confirm({
                        title: `删除「${row.name}」？`,
                        content: '磁盘文件会同时删除，无法恢复。',
                        okText: '删除',
                        okButtonProps: { danger: true },
                        cancelText: '取消',
                        onOk: () => deleteMutation.mutateAsync(row.id),
                      })
                    }}
                  >
                    删除
                  </Button>
                )}
              </div>
            ),
          },
        ]}
        filters={
          <>
            <Select
              value={kind}
              onChange={(value) => { setKind(value); setPage(1) }}
              style={{ width: 118 }}
              options={[
                { value: 'all', label: '全部类型' },
                ...FILE_KIND_OPTIONS.map((option) => ({ value: option.value, label: option.label })),
              ]}
            />
            <Input
              allowClear
              placeholder="搜索文件名 / MIME"
              value={keywordDraft}
              onChange={(event) => setKeywordDraft(event.target.value)}
              onPressEnter={() => { setKeyword(keywordDraft); setPage(1) }}
              style={{ minWidth: 220, flex: 1 }}
            />
            <Button onClick={() => { setKeyword(keywordDraft); setPage(1) }}>
              <Icon name="search" size={12} /> 筛选
            </Button>
          </>
        }
        toolbar={
          can('system:file:upload') ? (
            <Upload
              showUploadList={false}
              beforeUpload={(file) => {
                uploadMutation.mutate(file)
                return false
              }}
            >
              <Button type="primary" loading={uploadMutation.isPending}>
                <Icon name="upload" size={12} /> 上传
              </Button>
            </Upload>
          ) : undefined
        }
      />

      <Drawer
        open={!!shareFile}
        onClose={() => { setShareFile(undefined); setLatestShareURL('') }}
        width={620}
        title={shareFile ? `${shareFile.name} · 分享链接` : '分享链接'}
      >
        <div style={{ display: 'grid', gap: 14 }}>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center', justifyContent: 'space-between' }}>
            <span className="totp-meta">链接创建后只显示一次；撤销后立即失效。默认有效期 24 小时。</span>
            <Button
              type="primary"
              loading={createShareMutation.isPending}
              onClick={() => shareFile && createShareMutation.mutate(shareFile.id)}
            >
              生成新链接
            </Button>
          </div>

          {latestShareURL && (
            <div style={{ display: 'flex', gap: 8 }}>
              <Input readOnly value={latestShareURL} onFocus={(event) => event.currentTarget.select()} />
              <Button onClick={() => void copyLatestShare()}>复制</Button>
            </div>
          )}

          <div className="passkey-list">
            {sharesQuery.isLoading ? (
              <div className="passkey-empty">正在读取分享记录…</div>
            ) : (sharesQuery.data?.length ?? 0) === 0 ? (
              <div className="passkey-empty">还没有分享链接。</div>
            ) : (
              (sharesQuery.data ?? []).map((share) => {
                const revoked = !!share.revoked_at
                const expired = dayjs(share.expires_at).isBefore(dayjs())
                return (
                  <div className="passkey-item" key={share.id}>
                    <span className="passkey-icon">
                      <Icon name="login" size={15} />
                    </span>
                    <div className="passkey-info">
                      <b>
                        <span className={`tag ${revoked || expired ? 'orange' : 'green'}`}>
                          {revoked ? '已撤销' : expired ? '已过期' : '有效'}
                        </span>
                      </b>
                      <span>
                        创建于 {dayjs(share.created_at).format('MM-DD HH:mm')} · 到期{' '}
                        {dayjs(share.expires_at).format('MM-DD HH:mm')}
                      </span>
                    </div>
                    <div className="passkey-actions">
                      {!revoked && !expired && shareFile && (
                        <button
                          type="button"
                          className="danger"
                          onClick={() => {
                            modal.confirm({
                              title: '撤销这条分享链接？',
                              content: '撤销后链接立即失效，已经拿到的用户也无法继续访问。',
                              okText: '撤销',
                              okButtonProps: { danger: true },
                              cancelText: '取消',
                              onOk: () =>
                                revokeShareMutation.mutateAsync({ fileID: shareFile.id, shareID: share.id }),
                            })
                          }}
                        >
                          撤销
                        </button>
                      )}
                    </div>
                  </div>
                )
              })
            )}
          </div>
        </div>
      </Drawer>
    </>
  )
}
