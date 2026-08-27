INSERT INTO admin_permissions (name, description)
VALUES ('audit.export', 'Export redacted administrator audit events')
ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO admin_role_permissions (role_id, permission_name)
SELECT r.id, p.name
FROM admin_roles r
JOIN admin_permissions p ON p.name = 'audit.export'
WHERE r.name = 'super_admin'
ON CONFLICT DO NOTHING;
