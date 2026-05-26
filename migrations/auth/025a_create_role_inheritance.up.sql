CREATE TABLE role_inheritance (
    parent_role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    child_role_id  UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (parent_role_id, child_role_id),
    CHECK (parent_role_id <> child_role_id)
);

CREATE INDEX idx_role_inheritance_child
    ON role_inheritance (child_role_id);

INSERT INTO role_inheritance (parent_role_id, child_role_id)
SELECT parent.id, child.id
FROM (VALUES
    ('super_admin', 'admin'),
    ('admin', 'developer'),
    ('developer', 'analyst'),
    ('analyst', 'viewer')
) AS hierarchy(parent_name, child_name)
JOIN roles parent ON parent.name = hierarchy.parent_name
JOIN roles child ON child.name = hierarchy.child_name
ON CONFLICT DO NOTHING;

