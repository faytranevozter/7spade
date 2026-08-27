INSERT INTO admin_permissions (name, description)
VALUES ('users.sensitive.read', 'View user email addresses')
ON CONFLICT (name) DO NOTHING;

INSERT INTO admin_role_permissions (role_id, permission_name)
SELECT r.id, 'users.sensitive.read'
FROM admin_roles r
WHERE r.name = 'super_admin'
ON CONFLICT DO NOTHING;
