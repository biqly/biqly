-- email_block_list relocated to the dedicated mail database (bi_mail).
-- The mail worker now owns the block list; auth no longer references it.
DROP INDEX IF EXISTS email_block_list_blocked_at_idx;
DROP TABLE IF EXISTS email_block_list;
