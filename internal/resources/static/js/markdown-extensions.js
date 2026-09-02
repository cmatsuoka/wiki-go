/**
 * Markdown Extensions JavaScript
 * Handles interactive behavior for custom markdown extensions
 */

// Handle collapsible sections in print view
document.addEventListener('DOMContentLoaded', function() {
    // Find all collapsible sections
    const collapsibleSections = document.querySelectorAll('details.markdown-details');

    if (collapsibleSections.length > 0) {
        // Store original states
        const originalStates = new Map();

        // Add event listeners for print
        window.addEventListener('beforeprint', function() {
            // Store original states and open all sections before printing
            collapsibleSections.forEach(details => {
                originalStates.set(details, details.open);
                details.open = true;
            });
        });

        window.addEventListener('afterprint', function() {
            // Restore original states after printing
            collapsibleSections.forEach(details => {
                if (originalStates.has(details)) {
                    details.open = originalStates.get(details);
                }
            });
        });
    }

    // Chapter-links panel retract/expand toggle
    const chapterLinksPanel = document.querySelector('.chapter-links');
    const chapterLinksToggle = document.querySelector('.chapter-links-toggle');

    if (chapterLinksPanel && chapterLinksToggle) {
        const chapterLinksBody = chapterLinksPanel.querySelector('.chapter-links-body');
        const toggleText = chapterLinksToggle.querySelector('.chapter-links-toggle-text');

        function updateChapterLinksState(forceRetracted) {
            const isRetracted = typeof forceRetracted === 'boolean'
                ? forceRetracted
                : !chapterLinksPanel.classList.contains('retracted');

            chapterLinksPanel.classList.toggle('retracted', isRetracted);
            document.documentElement.classList.toggle('chapter-links-retracted', isRetracted);

            chapterLinksToggle.setAttribute('aria-expanded', String(!isRetracted));
            chapterLinksToggle.setAttribute('aria-label', isRetracted ? 'Expand chapter links' : 'Retract chapter links');

            if (toggleText) {
                toggleText.textContent = isRetracted ? '<<' : '>>';
            }

            if (chapterLinksBody) {
                chapterLinksBody.setAttribute('aria-hidden', String(isRetracted));
            }

            // Persist preference
            try {
                sessionStorage.setItem('chapter-links-retracted', String(isRetracted));
            } catch (e) {
                // Ignore storage errors
            }

            return isRetracted;
        }

        chapterLinksToggle.addEventListener('click', function() {
            updateChapterLinksState(!chapterLinksPanel.classList.contains('retracted'));
        });

        // Restore preference on page load (default to expanded). Use the two-step
        // transition trick to ensure the browser actually animates the retraction
        // when the page first renders.
        try {
            const savedRetracted = sessionStorage.getItem('chapter-links-retracted') === 'true';
            if (savedRetracted && !chapterLinksPanel.classList.contains('retracted')) {
                // Set the retracted state without animation first, then enable
                // the transition class so subsequent toggles animate smoothly.
                chapterLinksPanel.classList.add('no-animate');
                updateChapterLinksState(true);
                requestAnimationFrame(function() {
                    requestAnimationFrame(function() {
                        chapterLinksPanel.classList.remove('no-animate');
                    });
                });
            }
        } catch (e) {
            // Ignore storage errors
        }
    }
});