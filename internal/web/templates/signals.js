document.addEventListener('DOMContentLoaded', () => {
    const E = ExpenseOwl;
    let date = new Date();
    let expenses = [];
    let config = { categoryTargets: {} };
    let trendChart;
    let getOwner = E.bindOwnerRail(document.getElementById('ownerRail'), render);
    document.getElementById('prevMonth').addEventListener('click', () => move(-1));
    document.getElementById('nextMonth').addEventListener('click', () => move(1));

    async function move(offset) { date = new Date(date.getFullYear(), date.getMonth() + offset, 15); await load(); render(); }
    async function load() { const from = new Date(date.getFullYear(), date.getMonth() - 11, 1); const to = new Date(date.getFullYear(), date.getMonth() + 1, 1); expenses = await E.request(`/expenses?from=${encodeURIComponent(from.toISOString())}&to=${encodeURIComponent(to.toISOString())}`); }
    const ownerData = () => E.ownerExpenses(expenses, getOwner());
    const forMonth = month => ownerData().filter(item => E.inMonth(item, month));
    const spending = items => items.filter(item => item.amount < 0).reduce((sum, item) => sum + Math.abs(item.amount), 0);
    const income = items => items.filter(item => item.amount > 0).reduce((sum, item) => sum + item.amount, 0);

    function render() {
        document.getElementById('currentMonth').textContent = E.monthLabel(date);
        const current = forMonth(date);
        const previousDate = new Date(date.getFullYear(), date.getMonth() - 1, 15);
        const previous = forMonth(previousDate);
        const currentSpend = spending(current), previousSpend = spending(previous);
        const change = previousSpend ? Math.round((currentSpend - previousSpend) / previousSpend * 100) : null;
        const categories = {};
        current.filter(item => item.amount < 0).forEach(item => categories[item.category] = (categories[item.category] || 0) + Math.abs(item.amount));
        const history = Array.from({ length: 6 }, (_, index) => new Date(date.getFullYear(), date.getMonth() - 1 - index, 15)).map(month => spending(forMonth(month))).filter(Boolean);
        const average = history.length ? history.reduce((sum, value) => sum + value, 0) / history.length : 0;
        const targets = Object.entries(config.categoryTargets || {}).filter(([, value]) => value > 0);
        const within = targets.filter(([category, target]) => (categories[category] || 0) <= target).length;
        const cards = [
            { label: 'Month over month', value: change == null ? 'Not enough history' : `${Math.abs(change)}% ${change <= 0 ? 'lower' : 'higher'}`, text: change == null ? 'Add another month to unlock the comparison.' : `${E.euro(Math.abs(currentSpend - previousSpend))} ${change <= 0 ? 'less' : 'more'} spent than ${E.monthLabel(previousDate)}.` },
            { label: 'Target status', value: targets.length ? `${within} of ${targets.length} within target` : 'No targets set', text: targets.length ? `${targets.length-within} ${targets.length-within === 1 ? 'category is' : 'categories are'} over the monthly promise.` : 'Add monthly promises from Our setup.' }
        ];
        document.getElementById('signals').innerHTML = cards.map(card => `<article class="signal-card"><span class="eyebrow">${E.escape(card.label)}</span><h2 class="signal-number">${E.escape(card.value)}</h2><p>${E.escape(card.text)}</p></article>`).join('');
        document.getElementById('monthRows').innerHTML = Array.from({ length: 6 }, (_, index) => new Date(date.getFullYear(), date.getMonth() - index, 15)).map(month => { const amount = spending(forMonth(month)); const width = Math.min(100, average ? amount / Math.max(average, amount) * 100 : 0); return `<div class="category-row"><div><strong>${E.monthLabel(month)}</strong><div class="target-track"><span style="width:${width}%"></span></div></div><span class="money">${E.euro(amount)}</span></div>`; }).join('');
        renderTrend();
    }

    function renderTrend() {
        const periods = Array.from({ length: 12 }, (_, index) => new Date(date.getFullYear(), date.getMonth() - 11 + index, 15));
        const data = periods.map(period => ({ income: income(forMonth(period)), spending: spending(forMonth(period)) }));
        if (trendChart) trendChart.destroy();
        trendChart = new Chart(document.getElementById('trendChart'), { type: 'line', data: { labels: periods.map(period => period.toLocaleDateString(undefined, { month: 'short', year: '2-digit' })), datasets: [{ label: 'Income', data: data.map(item => item.income), borderColor: '#536b58', backgroundColor: 'rgba(117,140,119,.10)', fill: true, tension: .35 }, { label: 'Spending', data: data.map(item => item.spending), borderColor: '#a44859', backgroundColor: 'rgba(164,72,89,.07)', fill: true, tension: .35 }] }, options: { responsive: true, maintainAspectRatio: false, interaction: { intersect: false, mode: 'index' }, scales: { y: { beginAtZero: true, ticks: { callback: value => E.euro(value) } } } } });
    }

    (async () => { try { [config] = await Promise.all([E.request('/config'), load()]); Chart.defaults.color = '#51615a'; Chart.defaults.borderColor = '#dfe5de'; Chart.defaults.font.family = 'Avenir Next, Segoe UI, system-ui, sans-serif'; render(); } catch (error) { document.getElementById('signals').innerHTML = `<div class="signal-card">${E.escape(error.message)}</div>`; } })();
});
