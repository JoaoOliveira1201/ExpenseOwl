if ('serviceWorker' in navigator) {
    window.addEventListener('load', async () => {
        try {
            const registration = await navigator.serviceWorker.register('/sw.js', {
                scope: '/',
                updateViaCache: 'none'
            });
            await registration.update();
        } catch (error) {
            console.error('ExpenseOwl service worker registration failed:', error);
        }
    });
}
