-- Runs after 0000018 removed chats.project_id, so no foreign key still points
-- at this table.
DROP TABLE projects;
