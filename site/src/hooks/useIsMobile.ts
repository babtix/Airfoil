import { useState, useEffect } from 'react';

/**
 * Hook to detect whether the current viewport is mobile (≤ 768px) or tablet (769px - 1023px).
 * SSR-safe, responds dynamically to screen resize and orientation change.
 */
export function useIsMobile(breakpoint = 768) {
  const [isMobile, setIsMobile] = useState<boolean>(() => {
    if (typeof window === 'undefined') return false;
    return window.innerWidth <= breakpoint;
  });

  const [isTablet, setIsTablet] = useState<boolean>(() => {
    if (typeof window === 'undefined') return false;
    return window.innerWidth > 768 && window.innerWidth < 1024;
  });

  useEffect(() => {
    if (typeof window === 'undefined') return;

    const mediaQuery = window.matchMedia(`(max-width: ${breakpoint}px)`);
    const tabletQuery = window.matchMedia(`(min-width: 769px) and (max-width: 1023px)`);

    const updateMatches = () => {
      setIsMobile(mediaQuery.matches);
      setIsTablet(tabletQuery.matches);
    };

    updateMatches();

    // Modern listener with fallback
    if (mediaQuery.addEventListener) {
      mediaQuery.addEventListener('change', updateMatches);
      tabletQuery.addEventListener('change', updateMatches);
    } else {
      mediaQuery.addListener(updateMatches);
      tabletQuery.addListener(updateMatches);
    }

    return () => {
      if (mediaQuery.removeEventListener) {
        mediaQuery.removeEventListener('change', updateMatches);
        tabletQuery.removeEventListener('change', updateMatches);
      } else {
        mediaQuery.removeListener(updateMatches);
        tabletQuery.removeListener(updateMatches);
      }
    };
  }, [breakpoint]);

  return { isMobile, isTablet };
}
