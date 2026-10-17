import type { InjectionKey } from 'vue'
import type { OperationReceipt } from '@/api/types'

export type OperationReceiptLoader = (operationId: string) => Promise<OperationReceipt>
export const operationReceiptLoaderKey: InjectionKey<OperationReceiptLoader> = Symbol('operationReceiptLoader')
