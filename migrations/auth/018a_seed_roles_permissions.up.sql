-- Insert Default Roles
INSERT INTO roles (name, description) VALUES
  ('super_admin', 'Platform owner with full system access'),
  ('admin', 'Organization manager with administrative workspace access'),
  ('developer', 'Technical builder with access to semantic models and datasources'),
  ('analyst', 'Business user executing queries and dashboards'),
  ('viewer', 'Read-only viewer of dashboards and reports')
ON CONFLICT (name) DO NOTHING;

-- Insert Default Permissions
INSERT INTO permissions (name, resource, action, description) VALUES
  ('datasource:create', 'datasource', 'create', 'Create a new datasource connection'),
  ('datasource:read', 'datasource', 'read', 'View details of datasource connections'),
  ('datasource:update', 'datasource', 'update', 'Update configuration of datasource connections'),
  ('datasource:delete', 'datasource', 'delete', 'Delete datasource connections'),
  ('datasource:grant_access', 'datasource', 'grant_access', 'Grant datasource access to other users'),

  ('query:execute', 'query', 'execute', 'Execute queries against datasources'),
  ('query:compile', 'query', 'compile', 'Compile LogicalQuery to SQL'),
  ('query:share', 'query', 'share', 'Share query execution results'),

  ('model:create', 'model', 'create', 'Create a new semantic model'),
  ('model:read', 'model', 'read', 'Read semantic model definitions'),
  ('model:update', 'model', 'update', 'Update semantic model definitions'),
  ('model:delete', 'model', 'delete', 'Delete semantic models'),
  ('model:publish', 'model', 'publish', 'Publish semantic model drafts'),

  ('ai:query', 'ai', 'query', 'Submit natural language queries to AI'),
  ('ai:eval', 'ai', 'eval', 'Run AI evaluation benchmarks'),
  ('ai:settings', 'ai', 'settings', 'Configure AI models and parameters'),
  ('ai:queue:view_status', 'ai', 'view_status', 'View aggregate AI queue status'),
  ('ai:queue:view_details', 'ai', 'view_details', 'View details and prompts of other users in the queue'),

  ('admin:users', 'admin', 'users', 'Manage platform users'),
  ('admin:roles', 'admin', 'roles', 'Manage roles and permission matrix'),
  ('admin:audit', 'admin', 'audit', 'View platform audit logs'),
  ('admin:settings', 'admin', 'settings', 'Configure platform-wide settings'),
  ('admin:workspaces', 'admin', 'workspaces', 'Manage all workspaces'),

  ('workspace:create', 'workspace', 'create', 'Create new workspaces'),
  ('workspace:invite', 'workspace', 'invite', 'Invite members to workspace'),
  ('workspace:manage_datasources', 'workspace', 'manage_datasources', 'Attach or detach datasources to workspaces')
ON CONFLICT (name) DO NOTHING;

-- Map permissions to super_admin (all permissions)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'super_admin'
ON CONFLICT DO NOTHING;

-- Map permissions to admin
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'admin' AND p.name IN (
  'datasource:read', 'datasource:grant_access',
  'query:execute', 'query:compile', 'query:share',
  'model:create', 'model:read', 'model:update', 'model:delete', 'model:publish',
  'ai:query', 'ai:queue:view_status', 'ai:queue:view_details',
  'admin:users', 'admin:roles', 'admin:audit', 'admin:workspaces',
  'workspace:create', 'workspace:invite', 'workspace:manage_datasources'
)
ON CONFLICT DO NOTHING;

-- Map permissions to developer
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'developer' AND p.name IN (
  'datasource:create', 'datasource:read', 'datasource:update',
  'query:execute', 'query:compile', 'query:share',
  'model:create', 'model:read', 'model:update', 'model:delete', 'model:publish',
  'ai:query', 'ai:eval', 'ai:settings', 'ai:queue:view_status',
  'workspace:manage_datasources'
)
ON CONFLICT DO NOTHING;

-- Map permissions to analyst
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'analyst' AND p.name IN (
  'datasource:read',
  'query:execute', 'query:compile', 'query:share',
  'model:read',
  'ai:query', 'ai:queue:view_status'
)
ON CONFLICT DO NOTHING;

-- Map permissions to viewer
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'viewer' AND p.name IN (
  'datasource:read',
  'model:read',
  'ai:queue:view_status'
)
ON CONFLICT DO NOTHING;
