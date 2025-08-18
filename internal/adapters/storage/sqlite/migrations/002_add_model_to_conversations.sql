-- Add model field to conversations table for model preference per conversation

-- Add model column to conversations table
ALTER TABLE conversations ADD COLUMN model TEXT;

-- Create index for model field for query performance
CREATE INDEX IF NOT EXISTS idx_conversations_model ON conversations(model);