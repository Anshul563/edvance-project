-- Purchase source for commerce-driven enrollments. Paid orders
-- provision with source = 'purchase' through the internal enroll
-- endpoint; free/manual flows are unchanged.
ALTER TABLE enrollments DROP CONSTRAINT IF EXISTS enrollments_source_check;

ALTER TABLE enrollments
    ADD CONSTRAINT enrollments_source_check
        CHECK (
            source IN (
                'free',
                'manual',
                'purchase'
            )
        );
