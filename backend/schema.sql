-- “于是” PostgreSQL schema: the single source of truth for all database definitions.
-- Keep definitions in dependency order: extensions, types, tables, constraints,
-- indexes, triggers, then strictly necessary seed data.
--
-- No domain tables are defined yet because the product data model has not been
-- approved. Add the complete initial schema here after that design is accepted;
-- do not create parallel migration or per-feature schema files.

BEGIN;

-- Extensions

-- Types

-- Tables

-- Constraints and indexes

-- Triggers

-- Required seed data

COMMIT;
