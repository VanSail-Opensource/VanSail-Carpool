<script setup lang="ts">
import { AnimatePresence, motion } from 'motion-v'
import { computed } from 'vue'

import type { ConnectionNode } from '@/api/types'
import CountryFlag from '@/components/common/CountryFlag.vue'
import { motionDurations, motionSpring } from '@/composables/motionPresets'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { formatDateTime } from '@/utils/format'
import type { ConnectionTarget } from './types'

const props = withDefaults(defineProps<{
  nodes: readonly ConnectionNode[]
  disabled?: boolean
}>(), { disabled: false })

const emit = defineEmits<{ block: [target: ConnectionTarget] }>()
const { reducedMotion } = useMotionPreferences()
const activeNodes = computed(() => props.nodes.filter(node => node.ips.length > 0))

function select(node: ConnectionNode, index: number): void {
  const connection = node.ips[index]
  if (!connection) return
  emit('block', { nodeName: node.name, countryCode: node.countryCode, connection })
}
</script>

<template>
  <div class="connection-nodes">
    <AnimatePresence :initial="false" mode="popLayout">
      <motion.article
        v-for="node in activeNodes"
        :key="node.uuid"
        class="connection-node"
        layout
        :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 6 }"
        :animate="{ opacity: 1, y: 0 }"
        :exit="{ opacity: 0, y: reducedMotion ? 0 : -4 }"
        :transition="reducedMotion ? { duration: 0.08 } : motionSpring"
      >
        <header class="connection-node__header">
          <CountryFlag :code="node.countryCode" />
          <div>
            <h2>{{ node.name }}</h2>
            <p>{{ $t('connections.connectionCount', { count: node.ips.length }) }}</p>
          </div>
        </header>
        <div class="connection-node__ips">
          <motion.div
            v-for="(connection, index) in node.ips"
            :key="connection.handle"
            class="connection-ip"
            layout
            :transition="{ duration: reducedMotion ? 0.08 : motionDurations.normal, ease: 'easeOut' }"
          >
            <div class="connection-ip__identity">
              <UIcon name="i-ph-device-mobile" aria-hidden="true" />
              <div>
                <code>{{ connection.ip }}</code>
                <span>{{ $t('connections.lastSeen', { date: formatDateTime(connection.lastSeen) }) }}</span>
              </div>
            </div>
            <UButton
              type="button"
              color="error"
              variant="ghost"
              square
              icon="i-ph-shield-warning"
              :disabled="disabled"
              :aria-label="$t('connections.blockIp', { ip: connection.ip })"
              data-haptic="open"
              @click="select(node, index)"
            />
          </motion.div>
        </div>
      </motion.article>
    </AnimatePresence>
  </div>
</template>
