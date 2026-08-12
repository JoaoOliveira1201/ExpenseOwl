document.addEventListener('DOMContentLoaded', () => {
    const E = ExpenseOwl;
    let date = new Date();
    let expenses = [];
    let config = { categoryTargets: {} };
    let getOwner = E.bindOwnerRail(document.getElementById('ownerRail'), render);
    document.getElementById('prevMonth').addEventListener('click', () => move(-1));
    document.getElementById('nextMonth').addEventListener('click', () => move(1));

    async function move(offset) { date = new Date(date.getFullYear(), date.getMonth() + offset, 15); await load(); render(); }
    async function load() { const from = new Date(date.getFullYear(), date.getMonth() - 6, 1); const to = new Date(date.getFullYear(), date.getMonth() + 1, 1); expenses = await E.request(`/expenses?from=${encodeURIComponent(from.toISOString())}&to=${encodeURIComponent(to.toISOString())}`); }
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
        const largest = Object.entries(categories).sort((a,b) => b[1]-a[1])[0];
        const history = Array.from({ length: 6 }, (_, index) => new Date(date.getFullYear(), date.getMonth() - 1 - index, 15)).map(month => spending(forMonth(month))).filter(Boolean);
        const average = history.length ? history.reduce((sum, value) => sum + value, 0) / history.length : 0;
        const targets = Object.entries(config.categoryTargets || {}).filter(([, value]) => value > 0);
        const within = targets.filter(([category, target]) => (categories[category] || 0) <= target).length;
        const cards = [
            { label: 'Month over month', value: change == null ? 'Not enough history' : `${Math.abs(change)}% ${change <= 0 ? 'lower' : 'higher'}`, text: change == null ? 'Add another month to unlock the comparison.' : `${E.euro(Math.abs(currentSpend - previousSpend))} ${change <= 0 ? 'less' : 'more'} spent than ${E.monthLabel(previousDate)}.` },
            { label: 'Largest category', value: largest ? largest[0] : 'No spending', text: largest ? `${E.euro(largest[1])} accounts for ${currentSpend ? Math.round(largest[1]/currentSpend*100) : 0}% of this month’s outgoings.` : 'There are no outgoings in this view.' },
            { label: 'Six-month pace', value: average ? `${E.euro(Math.abs(currentSpend-average))} ${currentSpend <= average ? 'below' : 'above'}` : 'Building history', text: average ? `The recent monthly average is ${E.euro(average)}.` : 'More historical months will create a useful baseline.' },
            { label: 'Target status', value: targets.length ? `${within} of ${targets.length} within target` : 'No targets set', text: targets.length ? `${targets.length-within} ${targets.length-within === 1 ? 'category is' : 'categories are'} over the soft limit.` : 'Add soft category limits from Manage.' },
            { label: 'Income movement', value: E.euro(income(current)-income(previous)), text: `Current income is ${E.euro(income(current))}; the previous month was ${E.euro(income(previous))}.` },
            { label: 'Ledger activity', value: `${current.length} entries`, text: `${current.filter(item => item.amount < 0).length} outgoings and ${current.filter(item => item.amount > 0).length} income entries.` }
        ];
        document.getElementById('signals').innerHTML = cards.map(card => `<article class="signal-card"><span class="eyebrow">${E.escape(card.label)}</span><h2 class="signal-number">${E.escape(card.value)}</h2><p>${E.escape(card.text)}</p></article>`).join('');
        document.getElementById('monthRows').innerHTML = Array.from({ length: 6 }, (_, index) => new Date(date.getFullYear(), date.getMonth() - index, 15)).map(month => { const amount = spending(forMonth(month)); const width = Math.min(100, average ? amount / Math.max(average, amount) * 100 : 0); return `<div class="category-row"><div><strong>${E.monthLabel(month)}</strong><div class="target-track"><span style="width:${width}%"></span></div></div><span class="money">${E.euro(amount)}</span></div>`; }).join('');
    }
    (async () => { try { [config] = await Promise.all([E.request('/config'), load()]); render(); } catch (error) { document.getElementById('signals').innerHTML = `<div class="signal-card">${E.escape(error.message)}</div>`; } })();
});
