-- Cart lines carry a whole-unit quantity and mark confirmed running-low additions (ADR-0001).
ALTER TABLE user_product_state
 ADD COLUMN cart_quantity bigint NOT NULL DEFAULT 0,
 ADD COLUMN auto_added boolean NOT NULL DEFAULT false;
UPDATE user_product_state SET cart_quantity=1 WHERE in_cart;
ALTER TABLE user_product_state
 ADD CONSTRAINT cart_quantity_matches_membership CHECK (
  (in_cart AND cart_quantity BETWEEN 1 AND 999) OR (NOT in_cart AND cart_quantity=0 AND NOT auto_added)
 );
