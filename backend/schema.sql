-- “于是” PostgreSQL schema: the single source of truth for all database definitions.
-- Keep definitions in dependency order: enum types, tables, constraints, indexes,
-- and comments.
--
-- No domain tables are defined yet because the product data model has not been
-- approved. Add the complete initial schema here after that design is accepted;
-- do not create parallel migration or per-feature schema files.
--
-- The P1 backend declarative workflow uses Atlas Community. Do not add extensions,
-- functions, triggers, row-level security, or seed DML until the schema deployment
-- strategy is reviewed. Those objects are not fully managed by the selected
-- Community feature set. Repository code must write updated_at values and outbox
-- rows explicitly in the same business transaction.

-- Enum types

-- Tables

-- Constraints and indexes

-- Comments
