import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App, Button, Form, Input, InputNumber, Select, Tag } from 'antd'
import { api } from '../api/client'
import type { DictItem, DictType, PageResult } from '../api/types'
import { useAuth } from '../stores/auth'
import { PageHeader } from '../components/PageHeader'
import { DataTable } from '../components/DataTable'
import { FormModal } from '../components/FormModal'
import { Icon } from '../components/Icon'
import { StatusTag } from '../components/StatusTag'

interface TypeValues {
  name: string
  code: string
  status: 0 | 1
  remark: string
}

interface ItemValues {
  label: string
  value: string
  sort: number
  status: 0 | 1
  tag_color: string
}

export function Dicts() {
  const { message, modal } = App.useApp()
  const queryClient = useQueryClient()
  const can = useAuth((state) => state.can)
  const [typePage, setTypePage] = useState(1)
  const [typePageSize, setTypePageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [keywordDraft, setKeywordDraft] = useState('')
  const [selectedType, setSelectedType] = useState<number>()
  const [typeOpen, setTypeOpen] = useState(false)
  const [editingType, setEditingType] = useState<DictType>()
  const [itemOpen, setItemOpen] = useState(false)
  const [editingItem, setEditingItem] = useState<DictItem>()
  const [typeForm] = Form.useForm<TypeValues>()
  const [itemForm] = Form.useForm<ItemValues>()

  const typesQuery = useQuery({
    queryKey: ['dict-types', typePage, typePageSize, keyword],
    queryFn: () => api.get<PageResult<DictType>>('/dict/types', { page: typePage, page_size: typePageSize, keyword }),
  })

  const itemsQuery = useQuery({
    queryKey: ['dict-items', selectedType],
    queryFn: () => api.get<DictItem[]>(`/dict/types/${selectedType}/items`),
    enabled: !!selectedType,
  })

  const types = typesQuery.data?.list ?? []

  useEffect(() => {
    if (!selectedType && types.length > 0) setSelectedType(types[0].id)
    if (selectedType && types.length > 0 && !types.some((type) => type.id === selectedType)) setSelectedType(types[0].id)
  }, [types, selectedType])

  const invalidateTypes = () => void queryClient.invalidateQueries({ queryKey: ['dict-types'] })
  const invalidateItems = () => void queryClient.invalidateQueries({ queryKey: ['dict-items'] })

  const typeMutation = useMutation({
    mutationFn: (values: TypeValues) =>
      editingType ? api.put<DictType>(`/dict/types/${editingType.id}`, values) : api.post<DictType>('/dict/types', values),
    onSuccess: () => {
      message.success(editingType ? '字典类型已更新' : '字典类型已创建')
      setTypeOpen(false)
      setEditingType(undefined)
      typeForm.resetFields()
      invalidateTypes()
    },
    onError: (error: Error) => message.error(error.message),
  })

  const itemMutation = useMutation({
    mutationFn: (values: ItemValues) =>
      editingItem
        ? api.put<DictItem>(`/dict/items/${editingItem.id}`, { ...values, dict_type_id: selectedType })
        : api.post<DictItem>(`/dict/types/${selectedType}/items`, { ...values, dict_type_id: selectedType }),
    onSuccess: () => {
      message.success(editingItem ? '字典项已更新' : '字典项已创建')
      setItemOpen(false)
      setEditingItem(undefined)
      itemForm.resetFields()
      invalidateItems()
    },
    onError: (error: Error) => message.error(error.message),
  })

  const deleteType = (type: DictType) => {
    modal.confirm({
      title: `删除字典「${type.name}」？`,
      content: '字典下的所有字典项会一并删除。',
      okText: '删除',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: async () => {
        try {
          await api.del(`/dict/types/${type.id}`)
          message.success('字典已删除')
          invalidateTypes()
        } catch (error) {
          message.error(error instanceof Error ? error.message : '删除失败')
        }
      },
    })
  }

  const deleteItem = async (item: DictItem) => {
    try {
      await api.del(`/dict/items/${item.id}`)
      message.success('字典项已删除')
      invalidateItems()
    } catch (error) {
      message.error(error instanceof Error ? error.message : '删除失败')
    }
  }

  return (
    <>
      <PageHeader
        title="字典管理"
        sub={`${typesQuery.data?.total ?? 0} 个字典类型 · 左侧选择类型，右侧维护字典项`}
        actions={
          can('system:dict:create') ? (
            <button className="btn primary" type="button" onClick={() => { setEditingType(undefined); typeForm.resetFields(); typeForm.setFieldsValue({ status: 1, remark: '' }); setTypeOpen(true) }}>
              <Icon name="plus" size={13} /> 新建字典
            </button>
          ) : undefined
        }
      />

      <div className="split-panels">
        <DataTable<DictType>
          storageKey="dict-types"
          rowKey="id"
          data={types}
          loading={typesQuery.isFetching}
          total={typesQuery.data?.total ?? 0}
          page={typePage}
          pageSize={typePageSize}
          onPageChange={(nextPage, nextSize) => { setTypePage(nextPage); setTypePageSize(nextSize) }}
          onRefresh={() => void typesQuery.refetch()}
          onRow={(row) => ({
            onClick: () => setSelectedType(row.id),
            style: {
              cursor: 'pointer',
              background: selectedType === row.id ? 'rgba(255, 138, 92, .09)' : undefined,
            },
          })}
          scroll={{ x: 'max-content' }}
          columns={[
            {
              title: '字典',
              key: 'name',
              render: (_, row) => (
                <div>
                  <b style={{ fontSize: 11 }}>{row.name}</b>
                  <div className="mono">{row.code}</div>
                </div>
              ),
            },
            { title: '状态', key: 'status', width: 70, render: (_, row) => <StatusTag status={row.status} /> },
            {
              title: '',
              key: 'actions',
              width: 90,
              render: (_, row) => (
                <div className="inline-actions">
                  {can('system:dict:update') && <Button size="small" type="text" onClick={() => { setEditingType(row); typeForm.setFieldsValue(row); setTypeOpen(true) }}>编辑</Button>}
                  {can('system:dict:delete') && <Button size="small" type="text" danger onClick={() => deleteType(row)}>删除</Button>}
                </div>
              ),
            },
          ]}
          filters={
            <>
              <Input
                allowClear
                placeholder="搜索字典名称 / 编码"
                value={keywordDraft}
                onChange={(event) => setKeywordDraft(event.target.value)}
                onPressEnter={() => { setKeyword(keywordDraft); setTypePage(1) }}
                style={{ minWidth: 160 }}
              />
              <Button onClick={() => { setKeyword(keywordDraft); setTypePage(1) }}>
                <Icon name="search" size={12} />
              </Button>
            </>
          }
          toolbar={
            can('system:dict:create') ? (
              <Button type="primary" onClick={() => { setEditingType(undefined); typeForm.resetFields(); typeForm.setFieldsValue({ status: 1, remark: '' }); setTypeOpen(true) }}>
                <Icon name="plus" size={12} /> 新建
              </Button>
            ) : undefined
          }
        />

        <DataTable<DictItem>
          storageKey="dict-items"
          rowKey="id"
          data={itemsQuery.data ?? []}
          loading={itemsQuery.isFetching}
          total={0}
          onRefresh={() => void itemsQuery.refetch()}
          scroll={{ x: 'max-content' }}
          columns={[
            { title: '标签', dataIndex: 'label', key: 'label', render: (value: string) => <b style={{ fontSize: 11 }}>{value}</b> },
            { title: '值', dataIndex: 'value', key: 'value', render: (value: string) => <span className="mono">{value}</span> },
            { title: '排序', dataIndex: 'sort', key: 'sort', width: 70 },
            {
              title: '样式',
              dataIndex: 'tag_color',
              key: 'tag_color',
              width: 100,
              render: (value: string) => value ? <Tag color={value}>{value}</Tag> : '—',
            },
            { title: '状态', dataIndex: 'status', key: 'status', width: 80, render: (value: 0 | 1) => <StatusTag status={value} /> },
            {
              title: '操作',
              key: 'actions',
              width: 130,
              render: (_, item) => (
                <div className="inline-actions">
                  {can('system:dict:update') && <Button size="small" type="text" onClick={() => { setEditingItem(item); itemForm.setFieldsValue(item); setItemOpen(true) }}>编辑</Button>}
                  {can('system:dict:delete') && <Button size="small" type="text" danger onClick={() => void deleteItem(item)}>删除</Button>}
                </div>
              ),
            },
          ]}
          toolbar={
            can('system:dict:create') ? (
              <Button type="primary" disabled={!selectedType} onClick={() => { setEditingItem(undefined); itemForm.resetFields(); itemForm.setFieldsValue({ status: 1, sort: 0, tag_color: '' }); setItemOpen(true) }}>
                <Icon name="plus" size={12} /> 新建项
              </Button>
            ) : undefined
          }
        />
      </div>

      <FormModal
        open={typeOpen}
        title={editingType ? '编辑字典' : '新建字典'}
        form={typeForm}
        onCancel={() => { setTypeOpen(false); setEditingType(undefined) }}
        onSubmit={(values) => typeMutation.mutate(values as unknown as TypeValues)}
        confirmLoading={typeMutation.isPending}
      >
        <div className="detail-grid">
          <Form.Item name="name" label="字典名称" rules={[{ required: true, message: '请输入字典名称' }]}>
            <Input placeholder="例如：用户状态" />
          </Form.Item>
          <Form.Item name="code" label="字典编码" rules={[{ required: true, message: '请输入字典编码' }]}>
            <Input placeholder="user_status" />
          </Form.Item>
          <Form.Item name="status" label="状态" initialValue={1}>
            <Select options={[{ label: '正常', value: 1 }, { label: '停用', value: 0 }]} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input placeholder="可选" />
          </Form.Item>
        </div>
      </FormModal>

      <FormModal
        open={itemOpen}
        title={editingItem ? '编辑字典项' : '新建字典项'}
        form={itemForm}
        onCancel={() => { setItemOpen(false); setEditingItem(undefined) }}
        onSubmit={(values) => itemMutation.mutate(values as unknown as ItemValues)}
        confirmLoading={itemMutation.isPending}
      >
        <div className="detail-grid">
          <Form.Item name="label" label="标签" rules={[{ required: true, message: '请输入标签' }]}>
            <Input placeholder="正常" />
          </Form.Item>
          <Form.Item name="value" label="值" rules={[{ required: true, message: '请输入值' }]}>
            <Input placeholder="1" />
          </Form.Item>
          <Form.Item name="sort" label="排序" initialValue={0}>
            <InputNumber min={0} max={9999} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="tag_color" label="标签颜色">
            <Select
              allowClear
              options={['green', 'blue', 'orange', 'red', 'purple', 'default'].map((color) => ({ label: color, value: color }))}
            />
          </Form.Item>
          <Form.Item name="status" label="状态" initialValue={1}>
            <Select options={[{ label: '正常', value: 1 }, { label: '停用', value: 0 }]} />
          </Form.Item>
        </div>
      </FormModal>
    </>
  )
}
