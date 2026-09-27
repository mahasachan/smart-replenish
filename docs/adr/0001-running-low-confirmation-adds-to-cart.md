# ADR-0001: A confirmed running-low question adds the product to the cart

- Status: Accepted
- Date: 2026-09-27

## Context

The MVP design only proposed suggestions. Accepting one added products to the shopping
list and never to the cart, and "automatic buying" was deferred. The proof of concept in
[issue #1](https://github.com/mahasachan/smart-replenish/issues/1) must show a different
loop. The backend predicts that a previously bought product is running low. The user
confirms whether that is true, and a confirmed item goes into the cart automatically. The
maintainer approved changing the "never cart" rule on the issue.

## Decision

- The production rules now ask instead of suggesting. When a prediction qualifies
  (remaining days ≤ 2 and confidence ≥ 0.7) and policy allows it, the engine chooses
  `ASK_IF_RUNNING_LOW`, which creates a "Running low on X?" question.
- A **yes** answer accepts the question. In the same transaction, it sets
  `in_cart = true` and `cart_quantity` to the median quantity per past purchase
  occasion, with a minimum of 1 and a maximum of 999. The cart line is marked
  `auto_added`, and an `ADDED_TO_CART` event is recorded with the question's decision
  reference. If the item is already in the cart, it is left unchanged.
- A **no / not yet** answer dismisses the question and snoozes further questions about
  that product. The snooze lasts half the product's median repurchase interval, with a
  minimum of one day. Policy blocks new questions during the snooze
  (`running_low_snoozed`). The prediction formula does not change.
- Answers are explicit labels. The metrics report running-low precision (yes among shown
  questions), the "not yet" rate, and purchases within seven days of a yes.
- Adding to the cart never checks out or creates a purchase. The user can change or
  remove the cart line. Jev stays shadow-only and cannot add anything to the cart.
- The decision context version is now `v2`. It adds
  `running_low_snooze_hours_remaining` and `median_quantity`.

## Alternatives

- **Add to the cart first, and treat removal as "not running low".** This was rejected.
  It changes the user's cart without consent, and a removal is a weaker label than an
  explicit answer.
- **Shift the prediction baseline on "not yet".** This was deferred. A snooze is simpler
  and keeps predictions explainable while the labels are collected.
- **Replace the list-suggestion flow.** This was rejected. `SUGGEST_NOW`, bundles and
  suggested-list proposals still work for other engines and keep their list semantics.

## Consequences

- Rules-vs-Jev agreement is now measured against `ASK_IF_RUNNING_LOW` rather than
  `SUGGEST_NOW`, so agreement history before this change is not directly comparable.
- `user_product_state` gains `cart_quantity` and `auto_added` (migration 002). Existing
  cart rows get a quantity of 1.
- Auto-adding to the cart makes authentication and ownership checks more urgent before
  any public exposure.
