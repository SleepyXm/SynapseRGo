"""Add email authentication to users.

Revision ID: 002_email_auth
Revises: 001_initial_schema
Create Date: 2026-08-15
"""

from typing import Sequence, Union

from alembic import op


revision: str = "002_email_auth"
down_revision: Union[str, Sequence[str], None] = "001_initial_schema"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    # Existing Synapse databases may already have the manually-added column.
    op.execute("ALTER TABLE public.users ADD COLUMN IF NOT EXISTS email VARCHAR(254)")
    op.execute("UPDATE public.users SET email = NULL WHERE email IS NOT NULL AND btrim(email) = ''")
    op.execute(
        "CREATE UNIQUE INDEX IF NOT EXISTS users_email_key "
        "ON public.users (email) WHERE email IS NOT NULL"
    )


def downgrade() -> None:
    op.execute("DROP INDEX IF EXISTS public.users_email_key")
    op.execute("ALTER TABLE public.users DROP COLUMN IF EXISTS email")
