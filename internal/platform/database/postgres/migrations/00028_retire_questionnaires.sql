-- +goose Up
-- Keep reward attribution and settled summaries before removing operational data.
INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at)
SELECT 'retired-questionnaire-reward:'||p.id,NULL,'commerce.retired_reward','user',p.user_id,
 json_build_object('participantId',p.id,'sourceId',p.questionnaire_id,'rewardMinor',q.reward_txb_minor,'awardedAt',p.awarded_at)::text,p.awarded_at
FROM questionnaire_participants p JOIN questionnaires q ON q.id=p.questionnaire_id
WHERE p.awarded_at IS NOT NULL ON CONFLICT(id) DO NOTHING;
INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at)
SELECT 'retired-questionnaire-import:'||id,NULL,'commerce.retired_settlement','retired_import',id,COALESCE(report_json,'{}'),updated_at
FROM questionnaire_imports WHERE status='settled' ON CONFLICT(id) DO NOTHING;

CREATE TEMP TABLE retired_questionnaire_operations(id TEXT PRIMARY KEY);
INSERT INTO retired_questionnaire_operations
SELECT id FROM provider_operations WHERE kind='questionnaire_settlement'
UNION SELECT operation.id FROM provider_operations operation
 JOIN provider_operation_items item ON item.operation_id=operation.id
 JOIN outbox_jobs job ON item.target_type='outbox_job' AND item.target_id=job.id
 WHERE operation.kind='outbox_retry' AND job.kind='questionnaire_settlement';
UPDATE provider_operation_items SET status='failed',error_code='FEATURE_RETIRED',
 completed_at=vansail_now(),updated_at=vansail_now()
WHERE operation_id IN (SELECT id FROM retired_questionnaire_operations)
 AND status IN ('queued','processing','pending_review');
UPDATE provider_operations SET status='failed',error_code='FEATURE_RETIRED',
 completed_at=vansail_now(),updated_at=vansail_now()
WHERE id IN (SELECT id FROM retired_questionnaire_operations)
 AND status IN ('queued','processing','pending_review','partial');
DELETE FROM outbox_jobs WHERE kind='questionnaire_settlement'
 OR (kind='provider_operation' AND json_extract(payload,'$.operationId') IN (SELECT id FROM retired_questionnaire_operations));
DROP TABLE retired_questionnaire_operations;
DROP TABLE questionnaire_imports;
DROP TABLE questionnaire_participants;
DROP TABLE questionnaires;

INSERT INTO schema_migrations(version,applied_at) VALUES('114_retire_questionnaires.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
