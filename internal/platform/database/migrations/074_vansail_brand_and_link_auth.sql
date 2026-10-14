-- Remove retired browser-login credentials while preserving browser sessions and handoffs.
DELETE FROM settings WHERE key='auth.browser.telegram';

-- Change only untouched seeded welcome documents; keep custom copy and consent revisions.
UPDATE onboarding_content SET draft_json=replace(draft_json,'TX Carpool','VanSail Carpool')
WHERE kind='welcome' AND json(draft_json)=json('{"en":[{"id":"hello","text":"Hi, how are you?"},{"id":"welcome","text":"Welcome to TX Carpool"},{"id":"setup","text":"It only takes a few seconds to complete setup."}],"zh-CN":[{"id":"hello","text":"\u4f60\u597d\uff0c\u6700\u8fd1\u600e\u4e48\u6837\uff1f"},{"id":"welcome","text":"\u6b22\u8fce\u6765\u5230 TX Carpool"},{"id":"setup","text":"\u53ea\u9700\u51e0\u79d2\u5373\u53ef\u5b8c\u6210\u8bbe\u7f6e\u3002"}]}');
UPDATE onboarding_content SET published_json=replace(published_json,'TX Carpool','VanSail Carpool')
WHERE kind='welcome' AND json(published_json)=json('{"en":[{"id":"hello","text":"Hi, how are you?"},{"id":"welcome","text":"Welcome to TX Carpool"},{"id":"setup","text":"It only takes a few seconds to complete setup."}],"zh-CN":[{"id":"hello","text":"\u4f60\u597d\uff0c\u6700\u8fd1\u600e\u4e48\u6837\uff1f"},{"id":"welcome","text":"\u6b22\u8fce\u6765\u5230 TX Carpool"},{"id":"setup","text":"\u53ea\u9700\u51e0\u79d2\u5373\u53ef\u5b8c\u6210\u8bbe\u7f6e\u3002"}]}');
