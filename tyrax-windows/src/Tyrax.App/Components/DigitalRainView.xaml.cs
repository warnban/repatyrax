using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using System.Windows.Threading;
using Brush = System.Windows.Media.Brush;
using SolidColorBrush = System.Windows.Media.SolidColorBrush;
using Color = System.Windows.Media.Color;
using FontFamily = System.Windows.Media.FontFamily;
using UserControl = System.Windows.Controls.UserControl;

namespace Tyrax.App.Components;

/// <summary>Матричный дождь на главном экране при активном туннеле.</summary>
public partial class DigitalRainView : UserControl
{
    public static readonly DependencyProperty IsActiveProperty =
        DependencyProperty.Register(
            nameof(IsActive), typeof(bool), typeof(DigitalRainView),
            new PropertyMetadata(false, OnActiveChanged));

    private static readonly char[] Chars = "TYRAX01アイウエオカキクケコサシスセソ".ToCharArray();
    private static readonly SolidColorBrush Red = new(Color.FromRgb(0xFF, 0x1E, 0x1E));
    private static readonly SolidColorBrush Dim = new(Color.FromRgb(0x6E, 0x6E, 0x6E));
    private static readonly SolidColorBrush White = new(Color.FromRgb(0xFF, 0xFF, 0xFF));

    private readonly DispatcherTimer _timer = new() { Interval = TimeSpan.FromMilliseconds(55) };
    private readonly List<Column> _columns = new();
    private readonly Random _rng = new();
    private double _colWidth = 14;
    private double _rowHeight = 16;

    public DigitalRainView()
    {
        InitializeComponent();
        Loaded += (_, _) => ResizeColumns();
        SizeChanged += (_, _) => ResizeColumns();
        _timer.Tick += (_, _) => Tick();
    }

    public bool IsActive
    {
        get => (bool)GetValue(IsActiveProperty);
        set => SetValue(IsActiveProperty, value);
    }

    private static void OnActiveChanged(DependencyObject d, DependencyPropertyChangedEventArgs e)
    {
        if (d is DigitalRainView view)
            view.SetActive((bool)e.NewValue);
    }

    private void SetActive(bool active)
    {
        if (active)
        {
            ResizeColumns();
            _timer.Start();
        }
        else
        {
            _timer.Stop();
            RainCanvas.Children.Clear();
            _columns.Clear();
        }
    }

    private void ResizeColumns()
    {
        if (!IsActive || ActualWidth < 1 || ActualHeight < 1) return;

        var count = Math.Max(8, (int)(ActualWidth / _colWidth));
        while (_columns.Count < count)
            _columns.Add(new Column(_rng.Next(0, (int)(ActualHeight / _rowHeight) + 8)));
        while (_columns.Count > count)
            _columns.RemoveAt(_columns.Count - 1);
    }

    private void Tick()
    {
        if (!IsActive || ActualHeight < 1) return;

        RainCanvas.Children.Clear();
        var maxRows = (int)(ActualHeight / _rowHeight) + 2;

        for (var i = 0; i < _columns.Count; i++)
        {
            var col = _columns[i];
            col.Y++;
            if (col.Y > maxRows + _rng.Next(4, 14))
            {
                col.Y = _rng.Next(-maxRows, 0);
                col.Head = _rng.Next(0, 3);
            }

            for (var r = 0; r < col.Head + 6; r++)
            {
                var y = col.Y - r;
                if (y < 0 || y > maxRows) continue;

                var opacity = Math.Max(0.04, 0.55 - r * 0.08);
                var tb = new TextBlock
                {
                    Text = Chars[_rng.Next(Chars.Length)].ToString(),
                    FontFamily = new FontFamily("Consolas"),
                    FontSize = 13,
                    FontWeight = r == 0 ? FontWeights.Black : FontWeights.Bold,
                    Foreground = PickBrush(r),
                    Opacity = opacity,
                };
                Canvas.SetLeft(tb, i * _colWidth);
                Canvas.SetTop(tb, y * _rowHeight);
                RainCanvas.Children.Add(tb);
            }
        }
    }

    private static Brush PickBrush(int distanceFromHead)
        => distanceFromHead switch
        {
            0 => Red,
            < 3 => White,
            _ => Dim,
        };

    private sealed class Column
    {
        public Column(int y) => Y = y;
        public int Y;
        public int Head = 2;
    }
}
