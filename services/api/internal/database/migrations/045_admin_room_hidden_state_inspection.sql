INSERT INTO admin_permissions (name, description)
VALUES ('rooms.inspect_hidden', 'Inspect hidden live game state')
ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO admin_role_permissions (role_id, permission_name)
SELECT r.id, p.name
FROM admin_roles r
JOIN admin_permissions p ON p.name = 'rooms.inspect_hidden'
WHERE r.name = 'super_admin'
ON CONFLICT DO NOTHING;
