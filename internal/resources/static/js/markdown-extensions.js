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

    // Chapter-links panel collapse/extend toggle
    const chapterLinksPanel = document.querySelector('.chapter-links');
    const chapterLinksToggle = document.querySelector('.chapter-links-toggle');

    if (chapterLinksPanel && chapterLinksToggle) {
        const chapterLinksBody = chapterLinksPanel.querySelector('.chapter-links-body');
        const toggleText = chapterLinksToggle.querySelector('.chapter-links-toggle-text');

        function updateChapterLinksState(forceCollapsed) {
            const isCollapsed = typeof forceCollapsed === 'boolean'
                ? forceCollapsed
                : chapterLinksPanel.classList.toggle('collapsed');

            chapterLinksPanel.classList.toggle('collapsed', isCollapsed);

            chapterLinksToggle.setAttribute('aria-expanded', String(!isCollapsed));
            chapterLinksToggle.setAttribute('aria-label', isCollapsed ? 'Expand chapter links' : 'Collapse chapter links');

            if (toggleText) {
                toggleText.textContent = isCollapsed ? '<<' : '>>';
            }

            if (chapterLinksBody) {
                chapterLinksBody.setAttribute('aria-hidden', String(isCollapsed));
            }

            // Persist preference
            try {
                localStorage.setItem('chapter-links-collapsed', String(isCollapsed));
            } catch (e) {
                // Ignore storage errors
            }

            return isCollapsed;
        }

        chapterLinksToggle.addEventListener('click', function() {
            updateChapterLinksState();
        });

        // Restore preference on page load (default to expanded). Use the two-step
        // transition trick to ensure the browser actually animates the retraction
        // when the page first renders.
        try {
            const savedCollapsed = localStorage.getItem('chapter-links-collapsed') === 'true';
            if (savedCollapsed && !chapterLinksPanel.classList.contains('collapsed')) {
                // Set the collapsed state without animation first, then enable
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