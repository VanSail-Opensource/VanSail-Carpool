-- +goose Up
-- Exact text timestamps and canonical payloads remain byte-preserving.
CREATE FUNCTION vansail_now() RETURNS text LANGUAGE sql VOLATILE AS $$
 SELECT to_char(clock_timestamp() AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
$$;
CREATE FUNCTION json_valid(value text) RETURNS boolean LANGUAGE sql IMMUTABLE AS $$
 SELECT COALESCE(value IS JSON, false)
$$;
CREATE FUNCTION json_type(value text) RETURNS text LANGUAGE sql IMMUTABLE STRICT AS $$
 SELECT CASE jsonb_typeof(value::jsonb)
 WHEN 'number' THEN CASE WHEN value::jsonb::text ~ '^-?[0-9]+$' THEN 'integer' ELSE 'real' END
 WHEN 'boolean' THEN value::jsonb::text
 ELSE jsonb_typeof(value::jsonb) END
$$;
CREATE FUNCTION instr(value text,needle text) RETURNS int4 LANGUAGE sql IMMUTABLE STRICT AS $$ SELECT strpos(value,needle) $$;
-- +goose StatementBegin
CREATE FUNCTION json_extract(value text,path text) RETURNS text LANGUAGE plpgsql IMMUTABLE STRICT AS $$
DECLARE result jsonb;
BEGIN
 IF path !~ '^\$\.[A-Za-z0-9_.]+$' THEN RAISE EXCEPTION 'unsupported JSON path'; END IF;
 result := value::jsonb #> string_to_array(substr(path,3),'.');
 IF jsonb_typeof(result)='null' THEN RETURN NULL; END IF;
 IF jsonb_typeof(result)='boolean' THEN RETURN CASE result::text WHEN 'true' THEN '1' ELSE '0' END; END IF;
 RETURN result #>> '{}';
END $$;
-- +goose StatementEnd
CREATE FUNCTION json_each(document text) RETURNS TABLE(key text,value text) LANGUAGE sql IMMUTABLE STRICT AS $$
 SELECT e.key,e.value #>> '{}' FROM jsonb_each(CASE WHEN jsonb_typeof(document::jsonb)='object' THEN document::jsonb ELSE '{}'::jsonb END) e
 UNION ALL SELECT (a.ordinality-1)::text,a.value #>> '{}' FROM jsonb_array_elements(CASE WHEN jsonb_typeof(document::jsonb)='array' THEN document::jsonb ELSE '[]'::jsonb END) WITH ORDINALITY a
$$;
CREATE FUNCTION vansail_json(value text) RETURNS jsonb LANGUAGE sql IMMUTABLE STRICT AS $$ SELECT value::jsonb $$;
CREATE FUNCTION json_set(value text,path text,replacement jsonb) RETURNS text LANGUAGE sql IMMUTABLE STRICT AS $$
 SELECT jsonb_set(value::jsonb,string_to_array(substr(path,3),'.'),replacement,true)::text
$$;
-- RFC 7396 merge-patch semantics used by provider completion documents.
-- +goose StatementBegin
CREATE FUNCTION json_patch(document text,patch text) RETURNS text LANGUAGE plpgsql IMMUTABLE STRICT AS $$
DECLARE result jsonb; item record;
BEGIN
 IF jsonb_typeof(patch::jsonb)<>'object' THEN RETURN patch::jsonb::text; END IF;
 result := CASE WHEN jsonb_typeof(document::jsonb)='object' THEN document::jsonb ELSE '{}'::jsonb END;
 FOR item IN SELECT * FROM jsonb_each(patch::jsonb) LOOP
  IF item.value='null'::jsonb THEN result:=result-item.key;
  ELSE result:=jsonb_set(result,ARRAY[item.key],json_patch(COALESCE(result->item.key,'null'::jsonb)::text,item.value::text)::jsonb,true);
  END IF;
 END LOOP;
 RETURN result::text;
END $$;
-- +goose StatementEnd
-- Only absolute ISO dates are accepted. Fixed parser settings make this safe in
-- expression indexes; relative inputs such as 'now' must never be indexed.
-- +goose StatementBegin
CREATE FUNCTION julianday(value text) RETURNS double precision LANGUAGE plpgsql IMMUTABLE STRICT
SET TimeZone='UTC' SET DateStyle='ISO,YMD' AS $$
BEGIN
 IF value !~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}([T ][0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})?)?$' THEN RETURN NULL; END IF;
 RETURN extract(epoch FROM value::timestamptz)::double precision/86400.0+2440587.5;
 EXCEPTION WHEN invalid_datetime_format OR datetime_field_overflow THEN RETURN NULL;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
