import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App, Button, Input, Select, Upload } from 'antd'
import dayjs from 'dayjs'
import { api, saveBlob } from '../api/client'
import type { FileItem, PageResult } from '../api/types'
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
            width: 150,
            fixed: 'right',
            render: (_, row) => (
              <div className="inline-actions">
                {can('system:file:download') && <Button size="small" type="text" onClick={() => void download(row)}>下载</Button>}
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
    </>
  )
}
