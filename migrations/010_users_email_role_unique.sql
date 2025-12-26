ALTER TABLE users
  DROP INDEX email;

ALTER TABLE users
  ADD UNIQUE KEY uq_users_email_role (email, role);

