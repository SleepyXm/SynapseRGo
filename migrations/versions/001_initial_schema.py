"""Baseline the existing Synapse schema.

Revision ID: 001_initial_schema
Revises:
Create Date: 2026-08-15
"""

from typing import Sequence, Union

from alembic import context, op
import sqlalchemy as sa
from sqlalchemy.dialects import postgresql


revision: str = "001_initial_schema"
down_revision: Union[str, Sequence[str], None] = None
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.execute("CREATE EXTENSION IF NOT EXISTS pgcrypto")
    inspector = None if context.is_offline_mode() else sa.inspect(op.get_bind())

    def table_missing(name: str) -> bool:
        return inspector is None or not inspector.has_table(name, schema="public")

    if table_missing("users"):
        op.create_table(
            "users",
            sa.Column("id", postgresql.UUID(as_uuid=True), nullable=False, server_default=sa.text("gen_random_uuid()")),
            sa.Column("username", sa.Text(), nullable=False),
            sa.Column("password", sa.Text(), nullable=False),
            sa.Column("favourites", postgresql.ARRAY(sa.Text()), nullable=True, server_default=sa.text("'{}'::text[]")),
            sa.Column("hf_tokens", postgresql.JSONB(astext_type=sa.Text()), nullable=True, server_default=sa.text("'[]'::jsonb")),
            sa.Column("created_at", sa.DateTime(timezone=True), nullable=True, server_default=sa.text("now()")),
            sa.Column("updated_at", sa.DateTime(timezone=True), nullable=True, server_default=sa.text("now()")),
            sa.PrimaryKeyConstraint("id", name="users_pkey"),
            sa.UniqueConstraint("username", name="users_username_key"),
            schema="public",
        )

    if table_missing("llms"):
        op.create_table(
            "llms",
            sa.Column("id", postgresql.UUID(as_uuid=True), nullable=False, server_default=sa.text("gen_random_uuid()")),
            sa.Column("name", sa.Text(), nullable=False),
            sa.Column("provider", sa.Text(), nullable=True),
            sa.PrimaryKeyConstraint("id", name="llms_pkey"),
            schema="public",
        )

    if table_missing("user_favorites"):
        op.create_table(
            "user_favorites",
            sa.Column("user_id", postgresql.UUID(as_uuid=True), nullable=False),
            sa.Column("llm_id", postgresql.UUID(as_uuid=True), nullable=False),
            sa.ForeignKeyConstraint(["llm_id"], ["public.llms.id"], name="user_favorites_llm_id_fkey", ondelete="CASCADE"),
            sa.ForeignKeyConstraint(["user_id"], ["public.users.id"], name="user_favorites_user_id_fkey", ondelete="CASCADE"),
            sa.PrimaryKeyConstraint("user_id", "llm_id", name="user_favorites_pkey"),
            schema="public",
        )

    if table_missing("conversations"):
        op.create_table(
            "conversations",
            sa.Column("id", postgresql.UUID(as_uuid=True), nullable=False, server_default=sa.text("gen_random_uuid()")),
            sa.Column("user_id", postgresql.UUID(as_uuid=True), nullable=True),
            sa.Column("llm_model", sa.String(), nullable=True),
            sa.Column("compressed_messages", sa.LargeBinary(), nullable=True),
            sa.Column("created_at", sa.DateTime(timezone=True), nullable=True, server_default=sa.text("now()")),
            sa.Column("updated_at", sa.DateTime(timezone=True), nullable=True, server_default=sa.text("now()")),
            sa.Column("title", sa.Text(), nullable=True),
            sa.ForeignKeyConstraint(["user_id"], ["public.users.id"], name="conversations_user_id_fkey", ondelete="CASCADE"),
            schema="public",
        )


def downgrade() -> None:
    op.drop_table("conversations", schema="public")
    op.drop_table("user_favorites", schema="public")
    op.drop_table("llms", schema="public")
    op.drop_table("users", schema="public")
