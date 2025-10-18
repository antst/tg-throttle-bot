-- Migration: 000007_remove_exemptions_table
-- Purpose: Remove unused exemptions table
-- Date: 2025-10-18
-- Feature: Database schema cleanup
--
-- Rationale:
--   - Table has 0 rows in production (never populated)
--   - No INSERT operations in codebase (no way to add exemptions)
--   - No admin commands to manage exemptions
--   - Functionality replaced by user_overrides table
--   - user_overrides has superior features (expiration, 3-state logic)
--   - Handler code calls exemption checks but always returns false
--
-- Safe: Table verified empty in production (SELECT COUNT(*) = 0)
--
-- Related: See docs/SCHEMA_CLEANUP_ANALYSIS.md for full analysis

DROP TABLE IF EXISTS exemptions CASCADE;
